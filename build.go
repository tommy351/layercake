package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ansel1/merry"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder/dockerignore"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
)

type BuildOptions struct {
	BuildArgs    []FlagMap `long:"build-arg" description:"Set build-time variables"`
	BuildKit     bool      `long:"build-kit" description:"Enable BuildKit (requires Docker 18.06+)" env:"DOCKER_BUILDKIT"`
	CgroupParent string    `long:"cgroup-parent" description:"Optional parent cgroup for the container"`
	CPUPeriod    int64     `long:"cpu-period" description:"Limit the CPU CFS (Completely Fair Scheduler) period"`
	CPUQuota     int64     `long:"cpu-quota" description:"Limit the CPU CFS (Completely Fair Scheduler) quota"`
	CPUSetCPUs   string    `long:"cpuset-cpus" description:"CPUs in which to allow execution (0-3, 0,1)"`
	CPUSetMems   string    `long:"cpuset-mems" description:"MEMs in which to allow execution (0-3, 0,1)"`
	CPUShares    int64     `long:"cpu-shares" description:"CPU shares (relative weight)"`
	DryRun       bool      `long:"dry-run" description:"Print Dockerfile only"`
	ForceRemove  bool      `long:"force-rm" description:"Always remove intermediate containers"`
	Isolation    string    `long:"isolation" description:"Container isolation technology"`
	Memory       int64     `long:"memory" description:"Memory limit"`
	MemorySwap   int64     `long:"memory-swap" description:"Swap limit equal to memory plus swap: '-1' to enable unlimited swap"`
	Network      string    `long:"network" description:" Set the networking mode for the RUN instructions during build" default:"default"`
	NoCache      bool      `long:"no-cache" description:"Do not use cache when building the image"`
	SecurityOpt  []string  `long:"security-opt" description:"Security options"`

	ctx             context.Context
	client          client.ImageAPIClient
	config          *Config
	basePath        string
	excludePatterns []string
	onlyBuilds      StringSet
	tempDir         string
	baseTarPath     string
	layerPaths      map[string]string
}

type imageManifest struct {
	Config   string
	RepoTags []string
	Layers   []string
}

func init() {
	var buildOptions BuildOptions

	_, err := parser.AddCommand("build", "Build images", "", &buildOptions)

	if err != nil {
		panic(err)
	}
}

func (b *BuildOptions) Execute(args []string) error {
	b.ctx = globalCtx
	b.basePath = cwd
	b.layerPaths = map[string]string{}

	if len(args) > 0 {
		b.onlyBuilds = NewStringSet()
		b.onlyBuilds.Insert(args...)
	}

	if err := b.initConfig(); err != nil {
		return merry.Wrap(err)
	}

	if b.DryRun {
		b.printDockerfiles()
		return nil
	}

	tempDir, err := ioutil.TempDir("", "layercake")

	if err != nil {
		return merry.Wrap(err)
	}

	b.tempDir = tempDir
	defer os.RemoveAll(tempDir)

	return RunSeries(
		b.initClient,
		b.loadIgnore,
		b.buildBaseTar,
		b.startBuild,
	)
}

func (b *BuildOptions) initClient() (err error) {
	b.client, err = NewDockerClient(b.ctx)
	return
}

func (b *BuildOptions) initConfig() (err error) {
	b.config, err = InitConfig()
	return
}

func (b *BuildOptions) loadIgnore() error {
	path := filepath.Join(b.basePath, ".dockerignore")
	file, err := os.Open(path)

	if err != nil {
		if os.IsNotExist(err) {
			logger.Debug("Unable to find an ignore file")
			return nil
		}

		logger.Error("Failed to open the ignore file")
		return merry.Wrap(err)
	}

	defer file.Close()

	patterns, err := dockerignore.ReadAll(file)

	if err != nil {
		logger.Error("Failed to read the ignore file")
		return merry.Wrap(err)
	}

	b.excludePatterns = patterns
	logger.Debug("Ignore file is loaded")
	return nil
}

func (b *BuildOptions) buildBaseTar() error {
	logger.Info("Building base context")

	b.baseTarPath = filepath.Join(b.tempDir, "base.tar")
	file, err := os.Create(b.baseTarPath)

	if err != nil {
		logger.Error("Failed to open a file")
		return merry.Wrap(err)
	}

	defer file.Close()

	reader, err := archive.TarWithOptions(b.basePath, &archive.TarOptions{
		Compression:     archive.Uncompressed,
		ExcludePatterns: b.excludePatterns,
	})

	if err != nil {
		logger.Error("Failed to build the base context")
		return merry.Wrap(err)
	}

	written, err := io.Copy(file, reader)

	if err != nil {
		logger.Error("Failed to write the base context")
		return merry.Wrap(err)
	}

	logger.WithField("size", written).Info("Base context is built")
	return nil
}

func (b *BuildOptions) startBuild() (err error) {
	b.config.SortBuilds().Range(func(name string, _ int) bool {
		if b.onlyBuilds != nil {
			skip := true

			b.onlyBuilds.Range(func(value string) bool {
				if name == value || b.config.FindDependencies(value).Contains(name) {
					skip = false
					return false
				}

				return true
			})

			if skip {
				return true
			}
		}

		build := b.config.Build[name]

		if err = b.buildImage(name, &build); err != nil {
			return false
		}

		return true
	})

	return merry.Wrap(err)
}

func (b *BuildOptions) buildImage(name string, build *BuildConfig) error {
	log := logger.WithField("prefix", name)
	dockerFile := []byte(build.Dockerfile())
	log.Info("Building the image")

	// Build tar
	file, err := os.Open(b.baseTarPath)

	if err != nil {
		logger.Error("Failed to open the base tar")
		return merry.Wrap(err)
	}

	defer file.Close()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	header := &tar.Header{
		Name:    path.Join(layercakeBaseDir, "Dockerfile"),
		Size:    int64(len(dockerFile)),
		ModTime: time.Now(),
		Mode:    0600,
	}

	// Write Dockerfile to tar
	if _, err := TarAddFile(tw, header, bytes.NewReader(dockerFile)); err != nil {
		log.Error("Failed to write Dockerfile to tar")
		return merry.Wrap(err)
	}

	// Write dependency to tar
	b.config.FindDependencies(name).Range(func(dep string) bool {
		var (
			file *os.File
			info os.FileInfo
		)

		if file, err = os.Open(filepath.Join(b.tempDir, b.layerPaths[dep])); err != nil {
			err = merry.Wrap(err)
			return false
		}

		defer file.Close()

		if info, err = file.Stat(); err != nil {
			err = merry.Wrap(err)
			return false
		}

		header := &tar.Header{
			Name:    path.Join(layercakeBaseDir, dep+".tar"),
			Size:    info.Size(),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
		}

		if _, err = TarAddFile(tw, header, file); err != nil {
			err = merry.Wrap(err)
			return false
		}

		return true
	})

	if err != nil {
		log.Error("Failed to import layers to tar")
		return merry.Wrap(err)
	}

	if err := tw.Flush(); err != nil {
		log.Error("Failed to flush the tar")
		return merry.Wrap(err)
	}

	// Build the image
	options := types.ImageBuildOptions{
		ForceRemove:  b.ForceRemove,
		Remove:       true,
		NoCache:      b.NoCache,
		BuildArgs:    map[string]*string{},
		CPUSetCPUs:   b.CPUSetCPUs,
		CPUSetMems:   b.CPUSetMems,
		CPUShares:    b.CPUShares,
		CPUQuota:     b.CPUQuota,
		CPUPeriod:    b.CPUPeriod,
		Memory:       b.Memory,
		MemorySwap:   b.MemorySwap,
		CgroupParent: b.CgroupParent,
		NetworkMode:  b.Network,
		SecurityOpt:  b.SecurityOpt,
		Isolation:    container.Isolation(b.Isolation),
		Dockerfile:   header.Name,
		CacheFrom:    build.CacheFrom,
		Labels:       build.Labels,
		Tags:         build.Tags,
	}

	if b.BuildKit {
		options.Version = types.BuilderBuildKit
	}

	for _, arg := range b.BuildArgs {
		options.BuildArgs[arg.Key] = arg.Value
	}

	for k, v := range build.Args {
		v := v
		options.BuildArgs[k] = &v
	}

	res, err := b.client.ImageBuild(b.ctx, io.MultiReader(&buf, file), options)

	if err != nil {
		log.Error("Failed to build the image")
		return merry.Wrap(err)
	}

	defer res.Body.Close()

	imgID, err := DisplayBuildStream(b.ctx, res.Body, os.Stdout, options.Version)

	if err != nil {
		log.Error("Failed to display response")
		return merry.Wrap(err)
	}

	log.WithField("id", imgID).Info("Image is built")

	if len(b.config.FindDependants(name)) == 0 {
		return nil
	}

	log.Info("Exporting the layer")

	// Save the image
	reader, err := b.client.ImageSave(b.ctx, []string{imgID})

	if err != nil {
		log.Error("Failed to save the image")
		return merry.Wrap(err)
	}

	defer reader.Close()

	var manifests []imageManifest
	tr := tar.NewReader(reader)

	for {
		header, err := tr.Next()

		if err == io.EOF {
			break
		}

		if header.Name == "manifest.json" {
			if manifests, err = b.decodeManifest(tr); err != nil {
				log.Error("Failed to parse the manifest")
				return merry.Wrap(err)
			}
		} else if strings.HasSuffix(header.Name, "/layer.tar") {
			if err := b.saveLayer(header, tr); err != nil {
				log.Error("Failed to save the layer")
				return merry.Wrap(err)
			}
		}
	}

	layers := manifests[0].Layers

	for i, layer := range layers {
		if i == len(layers)-1 {
			b.layerPaths[name] = layer
		} else {
			if err := os.Remove(filepath.Join(b.tempDir, layer)); err != nil {
				log.Error("Failed to remove unused layers")
				return merry.Wrap(err)
			}
		}
	}

	return nil
}

func (b *BuildOptions) decodeManifest(r io.Reader) (manifests []imageManifest, err error) {
	err = merry.Wrap(json.NewDecoder(r).Decode(&manifests))
	return
}

func (b *BuildOptions) saveLayer(header *tar.Header, r io.Reader) error {
	path := filepath.Join(b.tempDir, header.Name)

	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return merry.Wrap(err)
	}

	file, err := os.Create(path)

	if err != nil {
		return merry.Wrap(err)
	}

	defer file.Close()

	if _, err := io.Copy(file, r); err != nil {
		return merry.Wrap(err)
	}

	return nil
}

func (b *BuildOptions) printDockerfiles() {
	for name, build := range b.config.Build {
		logger.WithField("prefix", name).Info("Dockerfile")
		fmt.Println(build.Dockerfile())
	}
}
