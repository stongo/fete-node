resource "aws_ecs_service" "fete-node" {
  name            = "fete-node"
  cluster         = module.ecs.id
  requires_compatibilities = ["EC2"]
  task_definition = aws_ecs_task_definition.service.arn
  desired_count   = 3
  iam_role        = aws_iam_role.foo.arn
  depends_on      = [aws_iam_role_policy.foo]

  capacity_provider_strategy = {
    # On-demand instances
    signers_1 = {
      capacity_provider = module.ecs_cluster.autoscaling_capacity_providers["signers_1"].name
      weight            = 1
      base              = 1
    }
  }

  /*
  volume = {
    repo = {}
  }
  */

  ordered_placement_strategy {
    type  = "binpack"
    field = "cpu"
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.foo.arn
    container_name   = "fete-node"
    container_port   = local.api_port 
  }

  placement_constraints {
    type       = "memberOf"
    expression = "attribute:ecs.availability-zone in [us-east-2a, us-east-2b, us-east-2c]"
  }
}
