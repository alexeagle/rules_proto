package protobuf

import (
	"flag"
	"fmt"
	"log"
	"sort"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"

	pc "github.com/stackb/rules_proto/pkg/protoc"
)

// NewProtobufLanguage create a new ProtobufLanguage Gazelle extension implementation.
func NewProtobufLanguage(name string) *ProtobufLanguage {
	return &ProtobufLanguage{
		name:      name,
		rules:     pc.Rules(),
		providers: make(map[label.Label]pc.RuleProvider),
	}
}

// ProtobufLanguage implements language.Language.
type ProtobufLanguage struct {
	name  string
	rules pc.RuleRegistry
	// providers is a mapping from label -> the provider that produced the
	// rule. we save this in the config such that we can retrieve the
	// association later in the resolve step.
	providers map[label.Label]pc.RuleProvider
}

// Name returns the name of the language. This should be a prefix of the kinds
// of rules generated by the language, e.g., "go" for the Go extension since it
// generates "go_library" rules.
func (pl *ProtobufLanguage) Name() string { return pl.name }

// The following methods are implemented to satisfy the
// https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/resolve?tab=doc#Resolver
// interface, but are otherwise unused.
func (*ProtobufLanguage) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {
}

func (*ProtobufLanguage) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	return nil
}

func (*ProtobufLanguage) KnownDirectives() []string {
	return []string{
		pc.LanguageDirective,
		pc.PluginDirective,
		pc.RuleDirective,
	}
}

// Configure implements config.Configurer
func (pl *ProtobufLanguage) Configure(c *config.Config, rel string, f *rule.File) {
	if f == nil {
		return
	}
	if err := pl.getOrCreatePackageConfig(c.Exts).ParseDirectives(rel, f.Directives); err != nil {
		log.Fatalf("error while parsing rule directives in package %q: %v", rel, err)
	}
}

// Kinds returns a map of maps rule names (kinds) and information on how to
// match and merge attributes that may be found in rules of those kinds. All
// kinds of rules generated for this language may be found here.
func (*ProtobufLanguage) Kinds() map[string]rule.KindInfo {
	registry := pc.Rules()

	kinds := make(map[string]rule.KindInfo)
	for _, name := range registry.RuleNames() {
		rule, err := registry.LookupRule(name)
		if err != nil {
			log.Fatal("Kinds:", err)
		}
		kinds[rule.Name()] = rule.KindInfo()
	}

	return kinds
}

// Loads returns .bzl files and symbols they define. Every rule generated by
// GenerateRules, now or in the past, should be loadable from one of these
// files.
func (pl *ProtobufLanguage) Loads() []rule.LoadInfo {
	// Merge symbols
	symbolsByLoadName := make(map[string][]string)
	for _, name := range pl.rules.RuleNames() {
		rule, err := pl.rules.LookupRule(name)
		if err != nil {
			log.Fatal(err)
		}
		load := rule.LoadInfo()
		symbolsByLoadName[load.Name] = append(symbolsByLoadName[load.Name], load.Symbols...)
	}

	// Ensure names are sorted otherwise order of load statements can be
	// non-deterministic
	keys := make([]string, 0)
	for name := range symbolsByLoadName {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	// Build final load list
	loads := make([]rule.LoadInfo, 0)
	for _, name := range keys {
		symbols := symbolsByLoadName[name]
		sort.Strings(symbols)
		loads = append(loads, rule.LoadInfo{
			Name:    name,
			Symbols: symbols,
		})
	}
	return loads
}

// Fix repairs deprecated usage of language-specific rules in f. This is called
// before the file is indexed. Unless c.ShouldFix is true, fixes that delete or
// rename rules should not be performed.
func (*ProtobufLanguage) Fix(c *config.Config, f *rule.File) {}

// Imports returns a list of ImportSpecs that can be used to import the rule r.
// This is used to populate RuleIndex.
//
// If nil is returned, the rule will not be indexed. If any non-nil slice is
// returned, including an empty slice, the rule will be indexed.
func (pl *ProtobufLanguage) Imports(c *config.Config, r *rule.Rule, f *rule.File) []resolve.ImportSpec {
	srcs := r.AttrStrings("srcs")
	imports := make([]resolve.ImportSpec, len(srcs))

	for i, src := range srcs {
		imports[i] = resolve.ImportSpec{
			// Lang is the language in which the import string appears (this
			// should match Resolver.Name).
			Lang: pl.name,
			// Imp is an import string for the library.
			Imp: fmt.Sprintf("//%s:%s", f.Pkg, src),
		}
	}

	return imports
}

// Embeds returns a list of labels of rules that the given rule embeds. If a
// rule is embedded by another importable rule of the same language, only the
// embedding rule will be indexed. The embedding rule will inherit the imports
// of the embedded rule. Since SkyLark doesn't support embedding this should
// always return nil.
func (*ProtobufLanguage) Embeds(r *rule.Rule, from label.Label) []label.Label { return nil }

// Resolve translates imported libraries for a given rule into Bazel
// dependencies. Information about imported libraries is returned for each rule
// generated by language.GenerateRules in language.GenerateResult.Imports.
// Resolve generates a "deps" attribute (or the appropriate language-specific
// equivalent) for each import according to language-specific rules and
// heuristics.
func (pl *ProtobufLanguage) Resolve(
	c *config.Config,
	ix *resolve.RuleIndex,
	rc *repo.RemoteCache,
	r *rule.Rule,
	importsRaw interface{},
	from label.Label,
) {
	if provider, ok := pl.providers[from]; ok {
		provider.Resolve(c, r, importsRaw, from)
	}
}

// GenerateRules extracts build metadata from source files in a directory.
// GenerateRules is called in each directory where an update is requested in
// depth-first post-order.
//
// args contains the arguments for GenerateRules. This is passed as a struct to
// avoid breaking implementations in the future when new fields are added.
//
// A GenerateResult struct is returned. Optional fields may be added to this
// type in the future.
//
// Any non-fatal errors this function encounters should be logged using
// log.Print.
func (pl *ProtobufLanguage) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	cfg := pl.getOrCreatePackageConfig(args.Config.Exts)

	files := make(map[string]*pc.File)
	for _, f := range args.RegularFiles {
		if !pc.IsProtoFile(f) {
			continue
		}
		file := pc.NewFile(args.Rel, f)
		if err := file.Parse(); err != nil {
			log.Fatalf("unparseable proto file dir=%s, file=%s: %v", args.Dir, file.Basename, err)
		}
		files[f] = file
	}

	protoLibraries := make([]pc.ProtoLibrary, 0)
	for _, r := range args.OtherGen {
		if r.Kind() != "proto_library" {
			continue
		}
		srcs := r.AttrStrings("srcs")
		srcLabels := make([]label.Label, len(srcs))
		for i, src := range srcs {
			srcLabel, err := label.Parse(src)
			if err != nil {
				log.Fatalf("%s %q: unparseable source label %q: %v", r.Kind(), r.Name(), src, err)
			}
			srcLabels[i] = srcLabel
		}
		lib := pc.NewOtherProtoLibrary(r, matchingFiles(files, srcLabels)...)
		protoLibraries = append(protoLibraries, lib)
	}

	pkg := pc.NewPackage(args.Rel, cfg, protoLibraries...)

	for _, provider := range pkg.RuleProviders() {
		labl := label.New(args.Config.RepoName, args.Rel, provider.Name())
		pl.providers[labl] = provider
		// TODO: if needed allow FileVisitor to mutate the rule.File here.
	}

	rules := pkg.Rules()
	imports := make([]interface{}, len(rules))
	for i, rule := range rules {
		imports[i] = rule.PrivateAttr(config.GazelleImportsKey)
	}

	return language.GenerateResult{
		Gen:     rules,
		Imports: imports,
		Empty:   pkg.Empty(),
	}
}

// getOrCreatePackageConfig either inserts a new config into the map under the
// language name or replaces it with a clone.
func (pl *ProtobufLanguage) getOrCreatePackageConfig(exts map[string]interface{}) *pc.PackageConfig {
	var cfg *pc.PackageConfig
	if existingExt, ok := exts[pl.name]; ok {
		cfg = existingExt.(*pc.PackageConfig).Clone()
	} else {
		cfg = pc.NewPackageConfig()
	}
	exts[pl.name] = cfg
	return cfg
}

func matchingFiles(files map[string]*pc.File, srcs []label.Label) []*pc.File {
	matching := make([]*pc.File, 0)
	for _, src := range srcs {
		if file, ok := files[src.Name]; ok {
			matching = append(matching, file)
		}
	}
	return matching
}
