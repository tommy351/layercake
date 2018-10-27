package layercake

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver"
	"go.uber.org/zap"
)

type buildStep struct {
	Step
	Name string
}

type Builder struct {
	Context   context.Context
	Config    *Config
	ImageName string
	Logger    *zap.Logger

	tempDir string
}

func (b *Builder) Build() error {
	b.tempDir = ".layercake-tmp"

	if err := os.MkdirAll(b.tempDir, os.ModePerm); err != nil {
		return err
	}

	defer os.RemoveAll(b.tempDir)

	var steps []buildStep

	for name, step := range b.Config.Steps {
		steps = append(steps, buildStep{
			Step: step,
			Name: name,
		})
	}

	steps = sortSteps(steps)

	for _, step := range steps {
		if err := b.buildStep(&step); err != nil {
			return err
		}
	}

	return nil
}

func (b *Builder) buildStep(step *buildStep) error {
	if step.Image == "" {
		step.Image = fmt.Sprintf(b.ImageName, step.Name)
	}

	logger := b.Logger.With(zap.String("step", step.Name))

	logger.Info("Building image")

	if err := b.buildImage(step); err != nil {
		logger.Error("Failed to build the image", zap.Error(err))
		return err
	}

	logger.Info("Saving layer")

	if err := b.saveLayer(step); err != nil {
		logger.Error("Failed to save the layer", zap.Error(err))
		return err
	}

	return nil
}

func (b *Builder) buildImage(step *buildStep) error {
	args := []string{"build",
		// Image tag
		"-t", step.Image,
		// Read Dockerfile from stdin
		"-f", "-"}

	// Set build args
	for k, v := range step.Args {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, ".")

	// Build Dockerfile
	dockerFile := []string{"FROM " + step.From}

	dockerFile = append(dockerFile, step.Setup.Build())

	for _, dep := range step.DependsOn {
		dockerFile = append(dockerFile, fmt.Sprintf("ADD %s /", b.getLayerPath(dep)))
	}

	dockerFile = append(dockerFile, step.Build.Build())

	cmd := exec.CommandContext(b.Context, "docker", args...)
	cmd.Stdin = bytes.NewBufferString(strings.Join(dockerFile, "\n"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (b *Builder) saveLayer(step *buildStep) error {
	cmd := exec.CommandContext(b.Context, "docker", "save", step.Image)
	cmd.Stderr = os.Stderr
	out, err := cmd.StdoutPipe()

	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	tempDir := filepath.Join(b.tempDir, "image_"+step.Name)

	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		return err
	}

	defer os.RemoveAll(tempDir)

	if err := archiver.Tar.Read(out, tempDir); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	latestLayer := getLatestLayer(tempDir)

	if latestLayer == "" {
		return errors.New("unable to find the latest layer")
	}

	if err := os.Rename(filepath.Join(tempDir, latestLayer, "layer.tar"), b.getLayerPath(step.Name)); err != nil {
		return err
	}

	return nil
}

func (b *Builder) getLayerPath(name string) string {
	return path.Join(b.tempDir, fmt.Sprintf("layer_%s.tar", name))
}

func getLatestLayer(dir string) string {
	var output map[string]map[string]string
	file, err := os.Open(filepath.Join(dir, "repositories"))

	if err != nil {
		return ""
	}

	defer file.Close()

	if err := json.NewDecoder(file).Decode(&output); err != nil {
		return ""
	}

	for _, v := range output {
		return v["latest"]
	}

	return ""
}

func sortSteps(input []buildStep) []buildStep {
	var output []buildStep
	// The following maps use step.Name as keys
	// stepMap is a map of steps
	stepMap := map[string]buildStep{}
	// outputMap is to avoid duplication in the output array
	outputMap := map[string]bool{}
	// depMap saves dependencies of steps
	depMap := map[string]map[string]bool{}

	// Build maps
	for _, step := range input {
		stepMap[step.Name] = step

		// Insert to output if the step doesn't have any dependencies;
		// Otherwise, insert it to depMap.
		if len(step.DependsOn) == 0 {
			output = append(output, step)
			outputMap[step.Name] = true
		} else {
			depMap[step.Name] = map[string]bool{}

			for _, dep := range step.DependsOn {
				depMap[step.Name][dep] = true
			}
		}
	}

	for len(depMap) > 0 {
		for k, deps := range depMap {
			newDeps := map[string]bool{}

			// Remove dependencies which is in the output array
			for dep := range deps {
				if !outputMap[dep] {
					newDeps[dep] = true
				}
			}

			// Insert to output if dependencies is empty.
			if len(newDeps) == 0 {
				delete(depMap, k)
				output = append(output, stepMap[k])
				outputMap[k] = true
			} else {
				depMap[k] = newDeps
			}
		}
	}

	return output
}
