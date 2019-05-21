package build

import (
	"encoding/json"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
	"golang.org/x/xerrors"
)

func printStream(in io.Reader, out io.Writer) (string, error) {
	var id string
	fd, isTerm := term.GetFdInfo(out)

	err := jsonmessage.DisplayJSONMessagesStream(in, out, fd, isTerm, func(msg jsonmessage.JSONMessage) {
		var aux types.BuildResult

		if err := json.Unmarshal(*msg.Aux, &aux); err == nil {
			id = aux.ID
		}
	})

	if err != nil {
		return "", xerrors.Errorf("failed to display JSON message stream: %w", err)
	}

	return id, nil
}
