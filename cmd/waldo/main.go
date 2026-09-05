package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	comparepkg "github.com/mirage-security/waldo/internal/compare"
	"github.com/mirage-security/waldo/internal/config"
	"github.com/mirage-security/waldo/internal/model"
	"github.com/mirage-security/waldo/internal/policy"
	"github.com/mirage-security/waldo/internal/provider"
)

const usage = `Waldo joins deployment facts, code facts, policy, and human disposition.

Usage:
  waldo check [--root PATH] [--config PATH] [--facts PATH] [--json]
  waldo compare --base REPORT.json --head REPORT.json [--json]

Exit status 1 means policy failed. Exit status 2 means usage, configuration,
provider, or input failed.`

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	switch args[0] {
	case "check":
		return runCheck(ctx, args[1:], stdout, stderr)
	case "compare":
		return runCompare(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s\n", args[0], usage)
		return 2
	}
}

func runCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	rootFlag := flags.String("root", ".", "source tree to analyze")
	configFlag := flags.String("config", "waldo.yaml", "deployment model")
	factsFlag := flags.String("facts", "", "JSONL facts file; bypasses configured providers")
	jsonFlag := flags.Bool("json", false, "write a machine-readable report")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}

	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fmt.Fprintf(stderr, "resolve root: %v\n", err)
		return 2
	}
	configurationPath := resolveFromRoot(root, *configFlag)
	configuration, err := config.Load(configurationPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	var facts []model.CodeFact
	analysisInput := model.AnalysisInputProviders
	providerRuns := make([]model.ProviderRun, 0)
	if *factsFlag != "" {
		analysisInput = model.AnalysisInputFactsFile
		facts, err = provider.LoadFacts(resolveFromRoot(root, *factsFlag), root)
	} else {
		if len(configuration.Providers) == 0 {
			configuration.Providers = builtInProviders(configuration.Deployment)
		}
		var collection provider.Collection
		collection, err = provider.CollectWithSummary(ctx, root, configuration.Providers)
		facts = collection.Facts
		providerRuns = collection.Runs
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	findings, err := policy.Evaluate(configuration, facts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	report := model.Report{
		SchemaVersion: model.ReportSchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		Root:          root,
		Analysis: model.Analysis{
			Input:           analysisInput,
			ProviderRuns:    providerRuns,
			CodeFacts:       len(facts),
			DeploymentUnits: len(configuration.Deployment.Units),
			Policies:        len(configuration.Policies),
		},
		Findings: findings,
		Summary:  model.Summarize(findings),
	}
	if *jsonFlag {
		if err := writeJSON(stdout, report); err != nil {
			fmt.Fprintf(stderr, "write report: %v\n", err)
			return 2
		}
	} else {
		writeHumanReport(stdout, report)
	}
	if report.Summary.Failing > 0 {
		return 1
	}
	return 0
}

func builtInProviders(deployment config.Deployment) []config.Provider {
	roots := make(map[string]struct{})
	for _, unit := range deployment.Units {
		for _, root := range unit.CodeRoots {
			roots[filepath.ToSlash(filepath.Clean(root))] = struct{}{}
		}
	}
	sortedRoots := make([]string, 0, len(roots))
	for root := range roots {
		sortedRoots = append(sortedRoots, root)
	}
	sort.Strings(sortedRoots)

	command := []string{"waldo-javascript-provider"}
	for _, root := range sortedRoots {
		command = append(command, "--target", root)
	}
	return []config.Provider{{Name: "javascript", Command: command}}
}

func runCompare(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	baseFlag := flags.String("base", "", "base check JSON report")
	headFlag := flags.String("head", "", "head check JSON report")
	jsonFlag := flags.Bool("json", false, "write a machine-readable comparison")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *baseFlag == "" || *headFlag == "" {
		fmt.Fprintln(stderr, "compare requires --base and --head")
		return 2
	}
	base, err := readReport(*baseFlag)
	if err != nil {
		fmt.Fprintf(stderr, "read base report: %v\n", err)
		return 2
	}
	head, err := readReport(*headFlag)
	if err != nil {
		fmt.Fprintf(stderr, "read head report: %v\n", err)
		return 2
	}
	result := comparepkg.Reports(base, head)
	if *jsonFlag {
		if err := writeJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "write comparison: %v\n", err)
			return 2
		}
	} else {
		writeHumanComparison(stdout, result)
	}
	if result.Failing > 0 {
		return 1
	}
	return 0
}

func resolveFromRoot(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func readReport(path string) (model.Report, error) {
	file, err := os.Open(path)
	if err != nil {
		return model.Report{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var report model.Report
	if err := decoder.Decode(&report); err != nil {
		return model.Report{}, err
	}
	if report.SchemaVersion != 1 && report.SchemaVersion != model.ReportSchemaVersion {
		return model.Report{}, fmt.Errorf("schemaVersion must be 1 or %d", model.ReportSchemaVersion)
	}
	if err := ensureEOF(decoder); err != nil {
		return model.Report{}, err
	}
	report.Summary = model.Summarize(report.Findings)
	return report, nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("unexpected data after report")
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeHumanReport(writer io.Writer, report model.Report) {
	writeAnalysis(writer, "Analysis", report.Analysis)
	for _, finding := range report.Findings {
		fmt.Fprintf(writer, "%s %s %s %s:%d [%s]\n", strings.ToUpper(string(finding.Severity)), finding.PolicyID, finding.Message, finding.CodeFact.Source.Path, finding.CodeFact.Source.Line, finding.Disposition)
		fmt.Fprintf(writer, "  %s\n", finding.ID)
		if finding.DispositionReason != "" {
			fmt.Fprintf(writer, "  reason: %s\n", finding.DispositionReason)
		}
	}
	fmt.Fprintf(writer, "\n%d findings: %d unresolved, %d accepted, %d false-positive; %d failing\n", report.Summary.Total, report.Summary.Unresolved, report.Summary.Accepted, report.Summary.FalsePositives, report.Summary.Failing)
}

func writeHumanComparison(writer io.Writer, result comparepkg.Result) {
	writeAnalysis(writer, "Base analysis", result.BaseAnalysis)
	writeAnalysis(writer, "Head analysis", result.HeadAnalysis)
	for _, finding := range result.Introduced {
		fmt.Fprintf(writer, "+ %s %s %s:%d [%s]\n", strings.ToUpper(string(finding.Severity)), finding.PolicyID, finding.CodeFact.Source.Path, finding.CodeFact.Source.Line, finding.Disposition)
	}
	for _, finding := range result.Resolved {
		fmt.Fprintf(writer, "- %s %s %s:%d [%s]\n", strings.ToUpper(string(finding.Severity)), finding.PolicyID, finding.CodeFact.Source.Path, finding.CodeFact.Source.Line, finding.Disposition)
	}
	for _, change := range result.Changed {
		fmt.Fprintf(writer, "~ %s %s: %s/%s -> %s/%s\n", change.Head.PolicyID, change.Head.ID, change.Base.Severity, change.Base.Disposition, change.Head.Severity, change.Head.Disposition)
	}
	fmt.Fprintf(writer, "\n%d introduced, %d resolved, %d changed, %d unchanged; %d failing changes\n", len(result.Introduced), len(result.Resolved), len(result.Changed), result.Unchanged, result.Failing)
}

func writeAnalysis(writer io.Writer, label string, analysis model.Analysis) {
	if analysis.Input == "" {
		fmt.Fprintf(writer, "%s: unavailable in this report version.\n", label)
		return
	}
	if analysis.Input == model.AnalysisInputFactsFile {
		fmt.Fprintf(writer, "%s: loaded %d %s from a facts file; %d deployment %s; %d %s.\n",
			label,
			analysis.CodeFacts, plural(analysis.CodeFacts, "code fact", "code facts"),
			analysis.DeploymentUnits, plural(analysis.DeploymentUnits, "unit", "units"),
			analysis.Policies, plural(analysis.Policies, "policy", "policies"),
		)
		return
	}
	fmt.Fprintf(writer, "%s: %d %s completed; %d %s; %d deployment %s; %d %s.\n",
		label,
		len(analysis.ProviderRuns), plural(len(analysis.ProviderRuns), "provider", "providers"),
		analysis.CodeFacts, plural(analysis.CodeFacts, "code fact", "code facts"),
		analysis.DeploymentUnits, plural(analysis.DeploymentUnits, "unit", "units"),
		analysis.Policies, plural(analysis.Policies, "policy", "policies"),
	)
	for _, run := range analysis.ProviderRuns {
		fmt.Fprintf(writer, "  %s: %d %s\n", run.Name, run.CodeFacts, plural(run.CodeFacts, "code fact", "code facts"))
	}
}

func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
