package layercake

import "strings"

type Config struct {
	Steps map[string]Step `yaml:"steps"`
}

type Step struct {
	Image     string            `yaml:"image"`
	From      string            `yaml:"from"`
	Setup     ScriptSlice       `yaml:"setup"`
	Build     ScriptSlice       `yaml:"build"`
	Args      map[string]string `yaml:"args"`
	DependsOn []string          `yaml:"depends_on"`
}

type Script struct {
	Run     string `yaml:"run"`
	Arg     string `yaml:"arg"`
	WorkDir string `yaml:"workdir"`
	Copy    string `yaml:"copy"`
	Add     string `yaml:"add"`
}

func (s Script) Build() string {
	if s.Run != "" {
		return "RUN " + s.Run
	} else if s.Arg != "" {
		return "ARG " + s.Arg
	} else if s.WorkDir != "" {
		return "WORKDIR " + s.WorkDir
	} else if s.Copy != "" {
		return "COPY " + s.Copy
	} else if s.Arg != "" {
		return "ADD " + s.Add
	}

	return ""
}

type ScriptSlice []Script

func (s ScriptSlice) Build() string {
	var output []string

	for _, script := range s {
		output = append(output, script.Build())
	}

	return strings.Join(output, "\n")
}
