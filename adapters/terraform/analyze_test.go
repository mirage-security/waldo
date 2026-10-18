package terraform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirage-security/waldo/protocol"
)

func TestAnalyzeLocalECSModuleWithVarFile(t *testing.T) {
	root := t.TempDir()
	infra := filepath.Join(root, "infra")
	moduleDir := filepath.Join(infra, "modules", "service")
	mkdir(t, moduleDir)
	write(t, filepath.Join(infra, "main.tf"), `
variable "replicas" { type = number }
module "service" {
  source = "./modules/service"
  replicas = var.replicas
}
`)
	write(t, filepath.Join(infra, "production.tfvars"), "replicas = 3\n")
	write(t, filepath.Join(moduleDir, "main.tf"), `
variable "replicas" { type = number }
resource "aws_ecs_service" "this" {
  desired_count = var.replicas
  deployment_maximum_percent = 200
}
`)

	result, err := Analyze(protocol.DeploymentRequest{
		Root: root, Source: infra, Resource: "module.service",
		Options: map[string]any{"varFiles": []any{"production.tfvars"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["platform.executionModel"] != "orchestrated-container" || result.Facts["deployment.replicas.concurrent"] != true || result.Facts["deployment.replicas.maxConcurrent"] != 6 {
		t.Fatalf("unexpected ECS facts: %#v", result.Facts)
	}
}

func TestAnalyzeRepositoryTerraformExample(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "examples", "terraform-ecs-service", "infra")
	result, err := Analyze(protocol.DeploymentRequest{
		Root: root, Source: source, Resource: "aws_ecs_service.worker",
		Options: map[string]any{"varFiles": []any{"production.tfvars"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["deployment.replicas.concurrent"] != true || result.Facts["deployment.replicas.maxConcurrent"] != 4 {
		t.Fatalf("unexpected example facts: %#v", result.Facts)
	}
}

func TestAnalyzeSingleECSResourceWithoutRolloutOverlap(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "main.tf"), `
resource "aws_ecs_service" "worker" {
  desired_count = 1
  deployment_maximum_percent = 100
}
`)
	result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: "aws_ecs_service.worker"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["deployment.replicas.concurrent"] != false || result.Facts["deployment.replicas.maxConcurrent"] != 1 {
		t.Fatalf("unexpected concurrency: %#v", result.Facts)
	}
}

func TestAnalyzeECSModuleServicesMap(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "main.tf"), `
variable "replicas" { default = 1 }
module "service" {
  source = "terraform-aws-modules/ecs/aws"
  services = {
    worker = {
      enable_autoscaling = false
      desired_count = var.replicas
    }
  }
}
`)
	result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: "module.service"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["deployment.replicas.concurrent"] != true || result.Facts["deployment.replicas.maxConcurrent"] != 2 {
		t.Fatalf("default ECS rollout overlap was not represented: %#v", result.Facts)
	}
}

func TestAnalyzeLambdaResource(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "main.tf"), `
resource "aws_lambda_function" "relay" {
  function_name = "relay"
  reserved_concurrent_executions = 4
}
`)
	result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: "aws_lambda_function.relay"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["platform.executionModel"] != "request-scoped-function" || result.Facts["deployment.replicas.concurrent"] != true || result.Facts["deployment.replicas.maxConcurrent"] != 4 {
		t.Fatalf("unexpected Lambda facts: %#v", result.Facts)
	}
}

func TestAnalyzeLambdaModuleNullConcurrencyMeansUnreserved(t *testing.T) {
	root := t.TempDir()
	moduleDir := filepath.Join(root, "lambda")
	mkdir(t, moduleDir)
	write(t, filepath.Join(root, "main.tf"), `module "service" { source = "./lambda" }`)
	write(t, filepath.Join(moduleDir, "main.tf"), `
variable "concurrency" { default = null }
resource "aws_lambda_function" "this" {
  function_name = "relay"
  reserved_concurrent_executions = var.concurrency == null ? -1 : var.concurrency
}
`)
	result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: "module.service"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["deployment.replicas.concurrent"] != true {
		t.Fatalf("unreserved Lambda concurrency was not represented: %#v", result.Facts)
	}
	if _, exists := result.Facts["deployment.replicas.maxConcurrent"]; exists {
		t.Fatalf("unreserved Lambda received a guessed maximum: %#v", result.Facts)
	}
}

func TestAnalyzePreservesExplicitZeroECSCount(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "main.tf"), `
resource "aws_ecs_service" "cold" {
  desired_count = 0
}
`)
	result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: "aws_ecs_service.cold"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["deployment.replicas.concurrent"] != false || result.Facts["deployment.replicas.maxConcurrent"] != 0 {
		t.Fatalf("explicit zero count was replaced by a default: %#v", result.Facts)
	}
}

func TestAnalyzeUnknownResourceReturnsAuditableZeroFacts(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "main.tf"), `resource "aws_s3_bucket" "archive" { bucket = "archive" }`)
	result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: "aws_s3_bucket.archive"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Facts) != 0 {
		t.Fatalf("unsupported resource produced guessed facts: %#v", result.Facts)
	}
}

func TestAnalyzeDoesNotUseChildDefaultForUnknownCallerInput(t *testing.T) {
	root := t.TempDir()
	moduleDir := filepath.Join(root, "module")
	mkdir(t, moduleDir)
	write(t, filepath.Join(root, "main.tf"), `
data "aws_ssm_parameter" "replicas" { name = "/replicas" }
module "service" {
  source = "./module"
  replicas = data.aws_ssm_parameter.replicas.value
}
`)
	write(t, filepath.Join(moduleDir, "main.tf"), `
variable "replicas" { default = 3 }
resource "aws_ecs_service" "this" { desired_count = var.replicas }
`)
	result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: "module.service"})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := result.Facts["deployment.replicas.concurrent"]; exists {
		t.Fatalf("unknown caller value became a guessed count: %#v", result.Facts)
	}
}

func TestAnalyzeRejectsVarFileOutsideRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	mkdir(t, root)
	write(t, filepath.Join(root, "main.tf"), `resource "aws_ecs_service" "worker" { desired_count = 1 }`)
	write(t, filepath.Join(parent, "outside.tfvars"), "replicas = 3\n")
	_, err := Analyze(protocol.DeploymentRequest{
		Root: root, Source: root, Resource: "aws_ecs_service.worker",
		Options: map[string]any{"varFiles": []any{"../outside.tfvars"}},
	})
	if err == nil {
		t.Fatal("expected outside-root var file to fail")
	}
}

func TestAnalyzeSkipsDisabledModule(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "main.tf"), `
module "disabled" {
  count = 0
  source = "terraform-aws-modules/ecs/aws"
  services = { worker = { desired_count = 3 } }
}
`)
	result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: "module.disabled"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Facts) != 0 {
		t.Fatalf("disabled module produced deployment facts: %#v", result.Facts)
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Mirrors a vendored ECS module: services contain unresolved ARN fields, a
// for_each passes each.value through try(), and the service uses conditional count.
func TestAnalyzeVendoredModuleInstances(t *testing.T) {
	for _, replicas := range []int{0, 1, 3} {
		t.Run(fmt.Sprint(replicas), func(t *testing.T) {
			root := t.TempDir()
			vendor := filepath.Join(root, "vendor")
			service := filepath.Join(vendor, "service")
			mkdir(t, service)
			write(t, filepath.Join(root, "main.tf"), fmt.Sprintf(`
module "app" {
  source = "./vendor"
  services = {
    worker = {
      desired_count = %d
      deployment_maximum_percent = 200
      cluster_arn = aws_ecs_cluster.this.arn
    }
  }
}
`, replicas))
			write(t, filepath.Join(vendor, "main.tf"), `
variable "services" {}
module "service" {
  source = "./service"
  for_each = { for k, v in var.services : k => v }
  desired_count = try(each.value.desired_count, 99)
  deployment_maximum_percent = try(each.value.deployment_maximum_percent, 100)
  create = try(each.value.create, true)
  cluster_arn = try(each.value.cluster_arn, "fallback")
}
`)
			write(t, filepath.Join(service, "main.tf"), `
variable "desired_count" { default = 99 }
variable "deployment_maximum_percent" { default = 100 }
variable "create" { default = false }
variable "cluster_arn" { default = "fallback" }
locals { create_service = var.create }
resource "aws_ecs_service" "this" {
  count = local.create_service ? 1 : 0
  desired_count = var.desired_count
  deployment_maximum_percent = var.deployment_maximum_percent
}
# The supplied-but-unknown ARN must not become the child's default.
resource "aws_lambda_function" "wrong" {
  count = var.cluster_arn == "fallback" ? 1 : 0
}
`)
			for _, address := range []string{"module.app", "module.app.module.service", "module.app.module.service.aws_ecs_service.this"} {
				result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: address})
				if err != nil {
					t.Fatal(err)
				}
				if result.Facts["deployment.replicas.maxConcurrent"] != replicas*2 {
					t.Fatalf("%s: unexpected facts: %#v", address, result.Facts)
				}
			}
		})
	}
}

func TestUnknownVendoredReplicaCountDoesNotUseDefault(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	mkdir(t, child)
	write(t, filepath.Join(root, "main.tf"), `
module "service" {
  source = "./child"
  options = { desired_count = data.external.config.result.replicas }
}
`)
	write(t, filepath.Join(child, "main.tf"), `
variable "options" {}
resource "aws_ecs_service" "this" {
  desired_count = try(var.options.desired_count, 1)
}
`)
	result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: "module.service"})
	if err != nil {
		t.Fatal(err)
	}
	if _, guessed := result.Facts["deployment.replicas.maxConcurrent"]; guessed {
		t.Fatalf("unknown input used a fallback: %#v", result.Facts)
	}
	if result.Facts["platform.executionModel"] != "orchestrated-container" {
		t.Fatal(result.Facts)
	}
}

func TestPureFunctionsPreserveRuntimeUnknowns(t *testing.T) {
	for _, expression := range []string{
		"try(var.replicas, 1)",
		`try(tonumber("3"), 1)`,
		`can(tonumber("3")) ? 3 : 1`,
		`try(length(path.module), 1)`,
		`try(length(terraform.workspace), 1)`,
		"try(data.external.scale.result.replicas, 1)",
		"try(aws_ssm_parameter.scale.value, 1)",
		"try(module.scale.replicas, 1)",
		"can(data.external.scale.result.replicas) ? 3 : 1",
	} {
		t.Run(expression, func(t *testing.T) {
			root := t.TempDir()
			write(t, filepath.Join(root, "main.tf"), `
variable "replicas" {}
data "external" "scale" {}
resource "aws_ssm_parameter" "scale" {}
resource "aws_ecs_service" "this" { desired_count = `+expression+` }
`)
			result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: "aws_ecs_service.this"})
			if err != nil {
				t.Fatal(err)
			}
			if _, guessed := result.Facts["deployment.replicas.maxConcurrent"]; guessed {
				t.Fatalf("guessed unknown replicas: %#v", result.Facts)
			}
		})
	}
}

func TestPartiallyKnownLocalsReachFixedPoint(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "main.tf"), `
locals {
  options = { replicas = local.middle, arn = aws_ecs_cluster.this.arn }
  fallback = try(local.middle, 99)
  middle = local.final
  final = 3
}
resource "aws_ecs_cluster" "this" {}
resource "aws_ecs_service" "this" {
  desired_count = local.options.replicas
  deployment_maximum_percent = 100
}
resource "aws_ecs_service" "fallback" {
  desired_count = local.fallback
  deployment_maximum_percent = 100
}
`)
	for _, resource := range []string{"aws_ecs_service.this", "aws_ecs_service.fallback"} {
		result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: resource})
		if err != nil {
			t.Fatal(err)
		}
		if result.Facts["deployment.replicas.maxConcurrent"] != 3 {
			t.Fatalf("%s: %#v", resource, result.Facts)
		}
	}
}

func TestNestedModuleExpansionIsBounded(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	leaf := filepath.Join(child, "leaf")
	mkdir(t, leaf)
	entries := make([]string, 33)
	for i := range entries {
		entries[i] = fmt.Sprintf("k%d = %d", i, i)
	}
	collection := "{" + strings.Join(entries, ",") + "}"
	write(t, filepath.Join(root, "main.tf"), `module "app" {
source = "./child"
for_each = `+collection+`
}`)
	write(t, filepath.Join(child, "main.tf"), `module "leaf" {
source = "./leaf"
for_each = `+collection+`
}`)
	write(t, filepath.Join(leaf, "main.tf"), `resource "aws_ecs_service" "this" { desired_count = 1 }`)
	_, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: "module.app"})
	if err == nil || !strings.Contains(err.Error(), "exceeds 1024 static instances") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIndexedModuleAddressSelectsOnlyThatInstance(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	mkdir(t, child)
	write(t, filepath.Join(root, "main.tf"), `
module "service" {
  source = "./child"
  for_each = { "one.worker" = 2, other = 7 }
  replicas = each.value
}
`)
	write(t, filepath.Join(child, "main.tf"), `
variable "replicas" {}
resource "aws_ecs_service" "this" {
  desired_count = var.replicas
  deployment_maximum_percent = 100
}
`)
	for _, address := range []string{`module.service["one.worker"]`, `module.service["one.worker"].aws_ecs_service.this`} {
		result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: address})
		if err != nil {
			t.Fatal(err)
		}
		if result.Facts["deployment.replicas.maxConcurrent"] != 2 {
			t.Fatalf("%s: %#v", address, result.Facts)
		}
	}
	_, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: `module.service["missing"]`})
	if err == nil {
		t.Fatal("missing instance accepted")
	}
}

func TestVariableTypeDefaultsPrecedeTryFallbacks(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	mkdir(t, child)
	write(t, filepath.Join(root, "main.tf"), `
module "service" {
  source = "./child"
  options = {}
  rollout = null
}
`)
	write(t, filepath.Join(child, "main.tf"), `
variable "options" { type = object({ replicas = optional(number, 3) }) }
variable "rollout" {
  type = number
  default = 200
  nullable = false
}
resource "aws_ecs_service" "this" {
  desired_count = try(var.options.replicas, 1)
  deployment_maximum_percent = var.rollout
}
`)
	result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: root, Resource: "module.service"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["deployment.replicas.maxConcurrent"] != 6 {
		t.Fatal(result.Facts)
	}
}
