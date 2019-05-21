package build

import (
	"github.com/goombaio/dag"
	"github.com/goombaio/orderedmap"
	"github.com/tommy351/layercake/pkg/config"
	"golang.org/x/xerrors"
)

const rootVertexID = "@@ROOT"

func buildDAG(images map[string]config.BuildImage) (*dag.DAG, error) {
	result := dag.NewDAG()

	// Add vertices
	for k, v := range images {
		vertex := dag.NewVertex(k, v)

		if err := result.AddVertex(vertex); err != nil {
			return nil, xerrors.Errorf("failed to add vertex: %w", err)
		}
	}

	// Add edges
	for k, v := range images {
		vertex, err := result.GetVertex(k)

		if err != nil {
			return nil, xerrors.Errorf("failed to get vertex: %w", err)
		}

		for _, s := range v.Scripts {
			if s.Import != "" {
				target, err := result.GetVertex(s.Import)

				if err != nil {
					return nil, xerrors.Errorf("failed to get vertex: %w", err)
				}

				if err := result.AddEdge(target, vertex); err != nil {
					return nil, xerrors.Errorf("failed to add edge: %w", err)
				}
			}
		}
	}

	return result, nil
}

func addRootVertex(graph *dag.DAG, args []string) (*dag.Vertex, error) {
	root := dag.NewVertex(rootVertexID, nil)

	if err := graph.AddVertex(root); err != nil {
		return nil, xerrors.Errorf("failed to add vertex: %w", err)
	}

	for _, id := range args {
		target, err := graph.GetVertex(id)

		if err != nil {
			return nil, xerrors.Errorf("failed to get vertex: %w", err)
		}

		if err := graph.AddEdge(root, target); err != nil {
			return nil, xerrors.Errorf("failed to add edge: %w", err)
		}
	}

	return root, nil
}

func topologicalSort(root *dag.Vertex) []*dag.Vertex {
	om := orderedmap.NewOrderedMap()

	for _, child := range root.Children.Values() {
		vertex := child.(*dag.Vertex)

		for _, parent := range traverseUpToRoot(vertex) {
			om.Put(parent.ID, parent)
		}

		om.Put(vertex.ID, vertex)
	}

	result := make([]*dag.Vertex, om.Size())

	for i, v := range om.Values() {
		result[i] = v.(*dag.Vertex)
	}

	return result
}

func traverseUpToRoot(vertex *dag.Vertex) []*dag.Vertex {
	// nolint: prealloc
	var result []*dag.Vertex

	for _, parent := range vertex.Parents.Values() {
		p := parent.(*dag.Vertex)

		if p.ID == rootVertexID {
			break
		}

		result = append(result, traverseUpToRoot(p)...)
		result = append(result, p)
	}

	return result
}
