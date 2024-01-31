resource "aws_ecs_task_definition" "service" {
  family = "service"
  container_definitions = jsonencode([
    {
      name      = "fete-node"
      image     = "stongo/fete-node:latest"
      cpu       = 2 
      memory    = 2048 
      essential = true
      portMappings = [
        {
          containerPort = local.p2p_port 
          hostPort      = local.p2p_port
        },
        {
          containerPort = local.api_port 
          hostPort      = local.api_port
        },
      ]
    },
  ])

  volume {
    name      = "repo"
    host_path = "/var/lib/fete-node/"
  }

  placement_constraints {
    type       = "memberOf"
    expression = "attribute:ecs.availability-zone in [us-east-2a, us-east-2b, us-east-2c]"
  }
}
