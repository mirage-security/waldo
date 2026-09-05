package terraform

import (
	"os"
	"path/filepath"
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
