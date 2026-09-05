variable "desired_count" {
  type = number
}

resource "aws_ecs_service" "worker" {
  name                               = "expiry-worker"
  desired_count                      = var.desired_count
  deployment_maximum_percent         = 200
  deployment_minimum_healthy_percent = 100
}
