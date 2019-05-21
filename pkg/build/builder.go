package build

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
	"github.com/google/uuid"
	"github.com/google/wire"
	"github.com/goombaio/dag"
	"github.com/tommy351/layercake/pkg/config"
	"github.com/tommy351/layercake/pkg/docker"
	"github.com/tommy351/layercake/pkg/log"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
)

// Set provides everything required for a builder.
// nolint: gochecknoglobals
var Set = wire.NewSet(LoadIgnoreFile, NewBuilder)

type imageExport struct {
	ID    string
	Files []string
}

type imageManifest struct {
	Config   string
	RepoTags []string
	Layers   []string
}

type Builder struct {
	config           *config.Config
	client           docker.Client
	logger           *zap.Logger
	vertices         []*dag.Vertex
	exportMap        map[string]imageExport
	excludedPatterns ExcludedPatterns
}

func NewBuilder(conf *config.Config, client docker.Client, logger *zap.Logger, excludedPatterns ExcludedPatterns, args []string) (*Builder, error) {
	builder := &Builder{
		config:           conf,
		client:           client,
		logger:           logger,
		excludedPatterns: excludedPatterns,
		exportMap:        map[string]imageExport{},
	}

	// Build DAG
	graph, err := buildDAG(conf.Build.Images)

	if err != nil {
		return nil, xerrors.Errorf("failed to build DAG: %w", err)
	}

	// Build all images when args is empty
	if len(args) == 0 {
		for k := range conf.Build.Images {
			args = append(args, k)
		}
	}

	// Add a root vertex as the entry point of DAG
	root, err := addRootVertex(graph, args)

	if err != nil {
		return nil, xerrors.Errorf("failed to add root vertex: %w", err)
	}

	builder.vertices = topologicalSort(root)
	return builder, nil
}

func (b *Builder) Start() error {
	ctx := context.Background()

	for _, vertex := range b.vertices {
		img := vertex.Value.(config.BuildImage)
		logger := b.logger.With(zap.String("name", vertex.ID))
		ctx := log.NewContext(ctx, logger)

		logger.Info("Start building image")
		id, err := b.buildImage(ctx, &img)

		if err != nil {
			return xerrors.Errorf("failed to build image: %w", err)
		}

		logger = b.logger.With(zap.String("id", id))
		ctx = log.NewContext(ctx, logger)

		if vertex.Children.Size() > 0 {
			logger.Info("Finding exported files")
			exportedFiles, err := b.findImageExportedFiles(ctx, id)

			if err != nil {
				return xerrors.Errorf("failed to find exported files: %w", err)
			}

			b.exportMap[vertex.ID] = imageExport{
				ID:    id,
				Files: exportedFiles,
			}
		}
	}

	return nil
}

func (b *Builder) buildImage(ctx context.Context, image *config.BuildImage) (string, error) {
	logger := log.FromContext(ctx)

	logger.Debug("Generating Dockerfile")
	dockerFile := b.generateDockerFile(image)

	if b.config.Build.DryRun {
		fmt.Println(dockerFile)
		return "", nil
	}

	logger.Debug("Creating context tar")
	reader, err := b.newContextTar()

	if err != nil {
		return "", xerrors.Errorf("failed to create context tar: %w", err)
	}

	logger.Debug("Adding Dockerfile to context tar")
	reader, err = addDockerFileToTar(reader, []byte(dockerFile))

	if err != nil {
		return "", xerrors.Errorf("failed to add Dockerfile to context tar: %w", err)
	}

	options := types.ImageBuildOptions{
		Remove:     true,
		BuildArgs:  map[string]*string{},
		Dockerfile: dockerFilePath,
		CacheFrom:  image.CacheFrom,
		Labels:     image.Labels,
		Tags:       image.Tags,
		NoCache:    b.config.Build.NoCache,
		BuildID:    uuid.Must(uuid.NewRandom()).String(),
	}

	for k, v := range b.config.Build.Args {
		v := v
		options.BuildArgs[k] = &v
	}

	for k, v := range image.Args {
		v := v
		options.BuildArgs[k] = &v
	}

	logger.Debug("Building Docker image", zap.String("buildId", options.BuildID))
	res, err := b.client.ImageBuild(ctx, reader, options)

	if err != nil {
		return "", xerrors.Errorf("failed to build Docker image: %w", err)
	}

	defer func() {
		_ = res.Body.Close()
	}()

	id, err := printStream(res.Body, os.Stdout)

	if err != nil {
		return "", xerrors.Errorf("failed to print stream: %w", err)
	}

	logger.Info("Image built successfully", zap.String("id", id))
	return id, nil
}

func (b *Builder) generateDockerFile(image *config.BuildImage) string {
	var lines []string

	for _, s := range image.Scripts {
		if s.Import != "" {
			export := b.exportMap[s.Import]
			// nolint: gosec
			lines = append(lines, fmt.Sprintf("FROM %s AS %s", export.ID, s.Import))
		}
	}

	// nolint: gosec
	lines = append(lines, fmt.Sprintf("FROM %s", image.From))

	for _, s := range image.Scripts {
		switch {
		case s.Import != "":
			export, ok := b.exportMap[s.Import]

			if ok && len(export.Files) > 0 {
				lines = append(lines, fmt.Sprintf("COPY --from=%s %s /", s.Import, strings.Join(export.Files, " ")))
			} else {
				lines = append(lines, fmt.Sprintf("# No exported files from %q", s.Import))
			}

		case s.Instruction != "":
			lines = append(lines, s.Instruction+" "+s.Value)

		case s.Raw != "":
			lines = append(lines, s.Raw)
		}
	}

	return strings.Join(lines, "\n")
}

func (b *Builder) newContextTar() (io.Reader, error) {
	return archive.TarWithOptions(b.config.CWD, &archive.TarOptions{
		ExcludePatterns: b.excludedPatterns,
		Compression:     archive.Uncompressed,
	})
}

func (b *Builder) findImageExportedFiles(ctx context.Context, id string) ([]string, error) {
	logger := log.FromContext(ctx)

	logger.Debug("Saving Docker image to a tar archive")
	reader, err := b.client.ImageSave(ctx, []string{id})

	if err != nil {
		return nil, xerrors.Errorf("failed to save the image: %w", err)
	}

	defer func() {
		_ = reader.Close()
	}()

	var manifests []imageManifest
	tr := tar.NewReader(reader)
	layerFiles := map[string][]*tar.Header{}

	for {
		header, err := tr.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, xerrors.Errorf("failed to read tar: %w", err)
		}

		if header.Name == "manifest.json" {
			if err := json.NewDecoder(tr).Decode(&manifests); err != nil {
				return nil, xerrors.Errorf("failed to decode the manifest: %w", err)
			}

			logger.Debug("Decoded image manifest", zap.Any("manifest", manifests))
		} else if strings.HasSuffix(header.Name, "/layer.tar") {
			logger.Debug("Found layer.tar",
				zap.String("file", header.Name),
				zap.Int64("size", header.Size),
			)

			files, err := listTarHeaders(tar.NewReader(tr))

			if err != nil {
				return nil, xerrors.Errorf("failed to list files in tar: %w", err)
			}

			layerFiles[header.Name] = files
		}
	}

	var output []string
	layers := manifests[0].Layers
	lastLayer := layers[len(layers)-1]

	for _, header := range layerFiles[lastLayer] {
		if header.FileInfo().Mode().IsRegular() && !strings.HasPrefix(filepath.Base(header.Name), ".wh.") {
			output = append(output, "/"+header.Name)
		}
	}

	logger.Debug("Image extracted successfully", zap.String("layer", lastLayer))
	return output, nil
}
