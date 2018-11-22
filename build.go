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
	"strings"

	"github.com/ansel1/merry"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/sabhiram/go-gitignore"
	"go.uber.org/zap"
)

type BuildOptions struct {
	ForceRemove  bool              `long:"force-rm" description:"Always remove intermediate containers"`
	NoCache      bool              `long:"no-cache" description:"Do not use cache when building the image"`
	BuildArgs    map[string]string `long:"build-arg" description:"Set build-time variables"`
	CPUSetCPUs   string            `long:"cpuset-cpus" description:"CPUs in which to allow execution (0-3, 0,1)"`
	CPUSetMems   string            `long:"cpuset-mems" description:"MEMs in which to allow execution (0-3, 0,1)"`
	CPUShares    int64             `long:"cpu-shares" description:"CPU shares (relative weight)"`
	CPUQuota     int64             `long:"cpu-quota" description:"Limit the CPU CFS (Completely Fair Scheduler) quota"`
	CPUPeriod    int64             `long:"cpu-period" description:"Limit the CPU CFS (Completely Fair Scheduler) period"`
	Memory       int64             `long:"memory" description:"Memory limit"`
	MemorySwap   int64             `long:"memory-swap" description:"Swap limit equal to memory plus swap: '-1' to enable unlimited swap"`
	CgroupParent string            `long:"cgroup-parent" description:"Optional parent cgroup for the container"`
	Network      string            `long:"network" description:" Set the networking mode for the RUN instructions during build" default:"default"`
	DryRun       bool              `long:"dry-run" description:"Print Dockerfile only"`
	SecurityOpt  []string          `long:"security-opt" description:"Security options"`
	ShmSize      int64             `long:"shm-size" description:"Size of /dev/shm"`
	Args         struct {
		Path string
	} `positional-args:"yes"`

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
	b.imgLayers = map[string][]byte{}

	return runSeries(
		b.setBasePath,
		b.initClient,
		b.loadIgnore,
		b.buildBaseTar,
		b.startBuild,
	)
}

func (b *BuildOptions) setBasePath() (err error) {
	log := logger.With(zap.String("path", b.Args.Path))

	if b.basePath, err = filepath.Abs(b.Args.Path); err != nil {
		log.Error("Failed to resolve the absolute path", zap.Error(err))
		return
	}

	log.Debug("Base path is resolved", zap.String("base", b.basePath))
	return
}

func (b *BuildOptions) initClient() (err error) {
	log := logger

	if b.client, err = client.NewClientWithOpts(); err != nil {
		log.Error("Failed to initialize a Docker client", zap.Error(err))
		return
	}

	b.client.NegotiateAPIVersion(b.ctx)
	log.Debug("Docker client is initialized", zap.String("clientVersion", b.client.ClientVersion()))
	return
}

func (b *BuildOptions) loadIgnore() (err error) {
	path := filepath.Join(b.basePath, ".dockerignore")
	log := logger.With(zap.String("path", path))

	if b.ignore, err = ignore.CompileIgnoreFile(path); err != nil {
		if err == os.ErrNotExist {
			err = nil
			b.ignore = &ignore.GitIgnore{}
			log.Debug("Unable to find an ignore file")
			return
		}

		log.Error("Failed to load the ignore file", zap.Error(err))
		return
	}

	log.Debug("Ignore file is loaded")
	return
}

func (b *BuildOptions) buildBaseTar() error {
	var buf bytes.Buffer
	log := logger.With(zap.String("path", b.basePath))
	tw := tar.NewWriter(&buf)

	err := filepath.Walk(b.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		name := strings.TrimPrefix(filepath.ToSlash(strings.TrimPrefix(path, b.basePath)), "/")
		log := log.With(zap.String("name", name))

		// Determine if the path should be ignored
		if info.IsDir() {
			if b.ignore.MatchesPath(name + "/") {
				return filepath.SkipDir
			}

			return nil
		}

		if b.ignore.MatchesPath(name) {
			return nil
		}

		// Write the header
		header := &tar.Header{
			Name:    name,
			Size:    info.Size(),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
		}

		if err := tw.WriteHeader(header); err != nil {
			log.Error("Failed to write tar header", zap.Error(err))
			return err
		}

		// Copy the file
		file, err := os.Open(path)

		if err != nil {
			log.Error("Failed to open the file", zap.Error(err))
			return err
		}

		defer file.Close()

		written, err := io.Copy(tw, file)

		if err != nil {
			log.Error("Failed to copy file to tar", zap.Error(err))
			return err
		}

		log.Debug("File is written to tar", zap.Int64("size", written))
		return nil
	})

	if err != nil {
		log.Error("Failed to walk the directory", zap.Error(err))
		return err
	}

	if err := tw.Flush(); err != nil {
		log.Error("Failed to flush the tar writer", zap.Error(err))
		return err
	}

	b.baseTar = buf.Bytes()
	log.Debug("Base tar is built", zap.Int("size", buf.Len()))
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

	return nil
}

func (b *BuildOptions) buildImage(name string, build *BuildConfig) (err error) {
	log := logger.With(zap.String("name", name))
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
	if err = tw.WriteHeader(header); err != nil {
		log.Error("Failed to write tar header", zap.Error(err))
		return
	}

	if _, err = tw.Write(dockerFile); err != nil {
		log.Error("Failed to write Dockerfile to tar", zap.Error(err))
		return
	}

	// Write dependency to tar
	config.FindDependencies(name).Range(func(dep string) bool {
		layer := b.imgLayers[dep]
		header := &tar.Header{
			Name: fmt.Sprintf("%s/%s.tar", layercakeBaseDir, dep),
			Size: int64(len(layer)),
		}

		if err = tw.WriteHeader(header); err != nil {
			log.Error("Failed to write tar header", zap.Error(err))
			return false
		}

		if _, err = tw.Write(layer); err != nil {
			log.Error("Failed to write layer to tar", zap.Error(err))
			return false
		}

		return true
	})

	if err != nil {
		return
	}

	if err = tw.Close(); err != nil {
		log.Error("Failed to close tar", zap.Error(err))
		return
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
		ShmSize:      b.ShmSize,
		SecurityOpt:  b.SecurityOpt,
		Dockerfile:   header.Name,
		CacheFrom:    build.CacheFrom,
		Labels:       build.Labels,
	}

	if img := build.Image; img != "" {
		options.Tags = append(options.Tags, img)
	}

	for k, v := range b.BuildArgs {
		options.BuildArgs[k] = &v
	}

	for k, v := range build.Args {
		options.BuildArgs[k] = &v
	}

	res, err := b.client.ImageBuild(b.ctx, buf, options)

	if err != nil {
		log.Error("Failed to build the image", zap.Error(err))
		return err
	}

	var imgID string
	scanner := bufio.NewScanner(res.Body)
	defer res.Body.Close()

	for scanner.Scan() {
		var res buildResponse

		if err = json.Unmarshal(scanner.Bytes(), &res); err != nil {
			return
		}

		if id := res.Aux.ID; id != "" {
			imgID = id
		}

		if s := res.Stream; s != "" {
			fmt.Print(res.Stream)
		}

		if s := res.Error; s != "" {
			err = merry.New(s)
			log.Error("Failed to build the image", zap.Error(err))
			return
		}
	}

	if err = scanner.Err(); err != nil {
		log.Error("Failed to scan the response", zap.Error(err))
		return
	}

	log.Info("Image is built", zap.String("id", imgID))

	if len(config.FindDependants(name)) == 0 {
		return nil
	}

	log.Info("Exporting the layer")

	// Save the image
	reader, err := b.client.ImageSave(b.ctx, []string{imgID})

	if err != nil {
		log.Error("Failed to save the image", zap.Error(err))
		return err
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

		if header.Name == "manifest.json" {
			if err := json.NewDecoder(tr).Decode(&manifests); err != nil {
				log.Error("Failed to parse the manifest", zap.Error(err))
				return err
			}
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
				log.Error("Failed to copy the layer", zap.Error(err))
				return err
			}

			b.imgLayers[name] = layerBuf.Bytes()
			log.Debug("Layer is copied", zap.Int64("size", written))
		}
	}

	return nil
}
