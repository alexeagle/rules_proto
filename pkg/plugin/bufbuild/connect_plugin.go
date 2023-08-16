package bufbuild

import (
	"flag"
	"log"
	"path"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/stackb/rules_proto/pkg/protoc"
)

func init() {
	protoc.Plugins().MustRegisterPlugin(&ConnectProto{})
}

// ConnectProto implements Plugin for the bufbuild/connect-es plugin.
type ConnectProto struct{}

// Name implements part of the Plugin interface.
func (p *ConnectProto) Name() string {
	return "bufbuild:connect-es"
}

// Configure implements part of the Plugin interface.
func (p *ConnectProto) Configure(ctx *protoc.PluginContext) *protoc.PluginConfiguration {
	flags := parseConnectProtoOptions(p.Name(), ctx.PluginConfig.GetFlags())
	imports := make(map[string]bool)
	for _, file := range ctx.ProtoLibrary.Files() {
		for _, imp := range file.Imports() {
			imports[imp.Filename] = true
		}
	}
	var options []string
	for _, option := range ctx.PluginConfig.GetOptions() {
		// options may be configured to include many "M" options, but only
		// include the relevant ones to avoid BUILD file clutter.
		if strings.HasPrefix(option, "M") {
			keyVal := option[len("M"):]
			parts := strings.SplitN(keyVal, "=", 2)
			filename := parts[0]
			if !imports[filename] {
				continue
			}
		}
		options = append(options, option)
	}

	tsFiles := make([]string, 0)
	for _, file := range ctx.ProtoLibrary.Files() {
		tsFile := file.Name + ".pb.ts"
		if flags.excludeOutput[filepath.Base(tsFile)] {
			continue
		}
		if ctx.Rel != "" {
			tsFile = path.Join(ctx.Rel, tsFile)
		}
		tsFiles = append(tsFiles, tsFile)
	}

	pc := &protoc.PluginConfiguration{
		Label:   label.New("build_stack_rules_proto", "plugin/bufbuild", "es"),
		Outputs: protoc.DeduplicateAndSort(tsFiles),
		Options: protoc.DeduplicateAndSort(options),
	}
	if len(pc.Outputs) == 0 {
		pc.Outputs = nil
	}
	return pc
}

// ConnectProtoOptions represents the parsed flag configuration for the
// ConnectProto implementation.
type ConnectProtoOptions struct {
	excludeOutput map[string]bool
}

func parseConnectProtoOptions(kindName string, args []string) *ConnectProtoOptions {
	flags := flag.NewFlagSet(kindName, flag.ExitOnError)

	var excludeOutput string
	flags.StringVar(&excludeOutput, "exclude_output", "", "--exclude_output=foo.ts suppresses the file 'foo.ts' from the output list")

	if err := flags.Parse(args); err != nil {
		log.Fatalf("failed to parse flags for %q: %v", kindName, err)
	}
	config := &ConnectProtoOptions{
		excludeOutput: make(map[string]bool),
	}
	for _, value := range strings.Split(excludeOutput, ",") {
		config.excludeOutput[value] = true
	}

	return config
}
