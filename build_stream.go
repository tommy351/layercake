package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/ansel1/merry"
	"github.com/containerd/console"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
	"github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/progress/progressui"
	"golang.org/x/sync/errgroup"
)

func DisplayBuildStream(ctx context.Context, in io.Reader, out io.Writer, version types.BuilderVersion) (string, error) {
	switch version {
	case types.BuilderBuildKit:
		return displayBuildStreamBuildKit(ctx, in, out)
	default:
		return displayBuildStreamV1(in, out)
	}
}

func displayBuildStreamV1(in io.Reader, out io.Writer) (string, error) {
	var id string
	fd, isTerm := term.GetFdInfo(out)

	err := jsonmessage.DisplayJSONMessagesStream(in, out, fd, isTerm, func(msg jsonmessage.JSONMessage) {
		if auxID, err := parseBuildAuxID(&msg); err == nil {
			id = auxID
		}
	})

	return id, merry.Wrap(err)
}

func displayBuildStreamBuildKit(ctx context.Context, in io.Reader, out io.Writer) (string, error) {
	var id string
	statusCh := make(chan *client.SolveStatus)
	scanner := bufio.NewScanner(in)
	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		var c console.Console

		if file, ok := out.(*os.File); ok {
			if cons, err := console.ConsoleFromFile(file); err == nil {
				c = cons
			}
		}

		return progressui.DisplaySolveStatus(ctx, "", c, out, statusCh)
	})

	eg.Go(func() error {
		defer close(statusCh)

		for scanner.Scan() {
			var msg jsonmessage.JSONMessage

			if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
				continue
			}

			switch msg.ID {
			case "moby.image.id":
				if auxID, err := parseBuildAuxID(&msg); err == nil {
					id = auxID
				}

			case "moby.buildkit.trace":
				if status, err := parseBuildKitTrace(&msg); err == nil {
					statusCh <- status
				}
			}
		}

		return scanner.Err()
	})

	if err := eg.Wait(); err != nil {
		return "", err
	}

	return id, nil
}

func parseBuildAuxID(msg *jsonmessage.JSONMessage) (string, error) {
	var aux types.BuildResult

	if err := json.Unmarshal(*msg.Aux, &aux); err != nil {
		return "", merry.Wrap(err)
	}

	return aux.ID, nil
}

func parseBuildKitTrace(msg *jsonmessage.JSONMessage) (*client.SolveStatus, error) {
	var data []byte

	if err := json.Unmarshal(*msg.Aux, &data); err != nil {
		return nil, err
	}

	var resp moby_buildkit_v1.StatusResponse

	if err := resp.Unmarshal(data); err != nil {
		return nil, err
	}

	var status client.SolveStatus

	for _, v := range resp.Vertexes {
		status.Vertexes = append(status.Vertexes, &client.Vertex{
			Digest:    v.Digest,
			Inputs:    v.Inputs,
			Name:      v.Name,
			Started:   v.Started,
			Completed: v.Completed,
			Error:     v.Error,
			Cached:    v.Cached,
		})
	}

	for _, s := range resp.Statuses {
		status.Statuses = append(status.Statuses, &client.VertexStatus{
			ID:        s.ID,
			Vertex:    s.Vertex,
			Name:      s.Name,
			Total:     s.Total,
			Current:   s.Current,
			Timestamp: s.Timestamp,
			Started:   s.Started,
			Completed: s.Completed,
		})
	}

	for _, l := range resp.Logs {
		status.Logs = append(status.Logs, &client.VertexLog{
			Vertex:    l.Vertex,
			Stream:    int(l.Stream),
			Data:      l.Msg,
			Timestamp: l.Timestamp,
		})
	}

	return &status, nil
}
