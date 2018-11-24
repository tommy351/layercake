package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ansel1/merry"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/sabhiram/go-gitignore"
)

type BuildOptions struct {
	BuildArgs    map[string]string `long:"build-arg" description:"Set build-time variables"`
	CgroupParent string            `long:"cgroup-parent" description:"Optional parent cgroup for the container"`
	CPUPeriod    int64             `long:"cpu-period" description:"Limit the CPU CFS (Completely Fair Scheduler) period"`
	CPUQuota     int64             `long:"cpu-quota" description:"Limit the CPU CFS (Completely Fair Scheduler) quota"`
	CPUSetCPUs   string            `long:"cpuset-cpus" description:"CPUs in which to allow execution (0-3, 0,1)"`
	CPUSetMems   string            `long:"cpuset-mems" description:"MEMs in which to allow execution (0-3, 0,1)"`
	CPUShares    int64             `long:"cpu-shares" description:"CPU shares (relative weight)"`
	DryRun       bool              `long:"dry-run" description:"Print Dockerfile only"`
	ForceRemove  bool              `long:"force-rm" description:"Always remove intermediate containers"`
	Isolation    string            `long:"isolation" description:"Container isolation technology"`
	Memory       int64             `long:"memory" description:"Memory limit"`
	MemorySwap   int64             `long:"memory-swap" description:"Swap limit equal to memory plus swap: '-1' to enable unlimited swap"`
	Network      string            `long:"network" description:" Set the networking mode for the RUN instructions during build" default:"default"`
	NoCache      bool              `long:"no-cache" description:"Do not use cache when building the image"`
	SecurityOpt  []string          `long:"security-opt" description:"Security options"`

	ctx       context.Context
	client    *client.Client
	basePath  string
	ignore    *ignore.GitIgnore
	baseTar   []byte
	imgLayers map[string][]byte
}

type buildResponse struct {
	Stream string `json:"stream"`
	Aux    struct {
		ID string `json:"ID"`
	} `json:"aux"`
	Error string `json:"error"`
}

type imageManifest struct {
	Config   string
	RepoTags []string
	Layers   []string
}

var buildOptions BuildOptions

func init() {
	_, err := parser.AddCommand("build", "Build images", "", &buildOptions)

	if err != nil {
		panic(err)
	}
}

func (b *BuildOptions) Execute(args []string) error {
	b.ctx = globalCtx
	b.basePath = cwd
	b.imgLayers = map[string][]byte{}

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

func (b *BuildOptions) loadIgnore() (err error) {
	path := filepath.Join(b.basePath, ".dockerignore")

	if b.ignore, err = ignore.CompileIgnoreFile(path); err != nil {
		if err == os.ErrNotExist {
			b.ignore = &ignore.GitIgnore{}
			logger.Debug("Unable to find an ignore file")
			return nil
		}

		logger.Error("Failed to load the ignore file")
		return merry.Wrap(err)
	}

	logger.Debug("Ignore file is loaded")
	return
}

func (b *BuildOptions) buildBaseTar() error {
	logger.Info("Building base context")

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	if err := TarAddDir(tw, b.basePath, b.ignore); err != nil {
		logger.Error("Failed to build the base context")
		return merry.Wrap(err)
	}

	if err := tw.Flush(); err != nil {
		logger.Error("Failed to flush the tar")
		return merry.Wrap(err)
	}

	b.baseTar = buf.Bytes()
	logger.WithField("size", buf.Len()).Info("Base context is built")
	return nil
}

func (b *BuildOptions) startBuild() (err error) {
	config.SortBuilds().Range(func(name string, _ int) bool {
		build := config.Build[name]

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

	// Dry run: only print Dockerfile
	if b.DryRun {
		log.Info("Dockerfile")
		fmt.Println(string(dockerFile))
		return nil
	}

	log.Info("Building the image")

	// Build tar
	buf := bytes.NewBuffer(b.baseTar)
	tw := tar.NewWriter(buf)
	header := &tar.Header{
		Name: layercakeBaseDir + "/Dockerfile",
		Size: int64(len(dockerFile)),
	}

	// Write Dockerfile to tar
	if _, err := TarAddFile(tw, header, bytes.NewReader(dockerFile)); err != nil {
		log.Error("Failed to write Dockerfile to tar")
		return merry.Wrap(err)
	}

	// Write dependency to tar
	var err error

	config.FindDependencies(name).Range(func(dep string) bool {
		layer := b.imgLayers[dep]
		header := &tar.Header{
			Name: fmt.Sprintf("%s/%s.tar", layercakeBaseDir, dep),
			Size: int64(len(layer)),
		}

		if _, err = TarAddFile(tw, header, bytes.NewReader(layer)); err != nil {
			log.Error("Failed to write the layer to tar")
			return false
		}

		return true
	})

	if err != nil {
		return merry.Wrap(err)
	}

	if err := tw.Close(); err != nil {
		log.Error("Failed to close the tar")
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
	}

	if img := build.Image; img != "" {
		options.Tags = append(options.Tags, img)
	}

	for k, v := range b.BuildArgs {
		v := v
		options.BuildArgs[k] = &v
	}

	for k, v := range build.Args {
		v := v
		options.BuildArgs[k] = &v
	}

	res, err := b.client.ImageBuild(b.ctx, buf, options)

	if err != nil {
		log.Error("Failed to build the image")
		return merry.Wrap(err)
	}

	var imgID string
	scanner := bufio.NewScanner(res.Body)
	defer res.Body.Close()

	for scanner.Scan() {
		var res buildResponse

		if err := json.Unmarshal(scanner.Bytes(), &res); err != nil {
			return merry.Wrap(err)
		}

		if id := res.Aux.ID; id != "" {
			imgID = id
		}

		if s := res.Stream; s != "" {
			fmt.Print(res.Stream)
		}

		if s := res.Error; s != "" {
			log.Error("Failed to build the image")
			return merry.New(s)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error("Failed to scan the response")
		return merry.Wrap(err)
	}

	log.WithField("id", imgID).Info("Image is built")

	if len(config.FindDependants(name)) == 0 {
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

	var tarBuf bytes.Buffer
	teeReader := io.TeeReader(reader, &tarBuf)

	// First read: find manifest
	var manifests []imageManifest
	tr := tar.NewReader(teeReader)

	for {
		header, err := tr.Next()

		if err == io.EOF {
			break
		}

		if header.Name != "manifest.json" {
			continue
		}

		if err := json.NewDecoder(tr).Decode(&manifests); err != nil {
			log.Error("Failed to parse the manifest")
			return merry.Wrap(err)
		}
	}

	// Second read: find the layer
	tr = tar.NewReader(&tarBuf)
	layers := manifests[0].Layers
	lastLayer := layers[len(layers)-1]

	for {
		header, err := tr.Next()

		if err == io.EOF {
			break
		}

		if header.Name == lastLayer {
			var layerBuf bytes.Buffer
			written, err := io.Copy(&layerBuf, tr)

			if err != nil {
				log.Error("Failed to copy the layer")
				return merry.Wrap(err)
			}

			b.imgLayers[name] = layerBuf.Bytes()
			log.WithField("size", written).Debug("Layer is exported")
		}
	}

	return nil
}
