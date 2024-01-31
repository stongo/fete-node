module "ecs_service" {
  source  = "terraform-aws-modules/ecs/aws//modules/service"
  version = "5.8.0"

  name                     = local.name 
  cluster_arn              = module.ecs.cluster_arn
  iam_role_name            = local.name 
  requires_compatibilities = ["EC2"]
  desired_count            = "3"
  depends_on               = [module.autoscaling]

  capacity_provider_strategy = {
    # On-demand instances
    signers_1 = {
      capacity_provider = module.ecs.autoscaling_capacity_providers["signers_1"].name
      weight            = 1
      base              = 1
    }
  }


  volume = {
    repo = {} 
  }

  container_definitions = {
    (local.container_name) = {
      image     = "stongo/fete-node:master"
      cpu       = 2
      memory    = 2048
      essential = true
      portMappings = [
        {
          name = "p2p"
          containerPort = local.p2p_port
          hostPort      = local.p2p_port
        },
        {
          name = "api"
          containerPort = local.api_port
          hostPort      = local.api_port
        },
      ]

      mount_points = [
        {
          sourceVolume  = "repo",
          containerPath = "/var/lib/fete-node"
        }
      ]
    },
  } 

  load_balancer = {
    target_group_arn = module.alb.target_groups["fete_network_ecs"].arn
    container_name   = local.container_name 
    container_port   = local.api_port
  }

  subnet_ids = module.vpc.private_subnets

  placement_constraints = {
    type       = "memberOf"
    expression = "attribute:ecs.availability-zone in [us-east-2a, us-east-2b, us-east-2c]"
  }

  security_group_rules = {
    alb_http_ingress = {
      type                     = "ingress"
      from_port                = local.api_port
      to_port                  = local.api_port
      protocol                 = "tcp"
      description              = "Service port"
      source_security_group_id = module.alb.security_group_id
    }
  }  
}
