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
      // @TODO: pin verion
      image     = "stongo/fete-node:master"
      command   = ["fete-node", "-repo=/var/lib/fete-node", "-host='0.0.0.0'"]]
      cpu       = 2
      memory    = 2048
      essential = true
      port_mappings = [
        {
          name          = "p2p"
          containerPort = local.p2p_port
          protocol      = "tcp"
        },
        {
          name          = "api"
          containerPort = local.api_port
          protocol      = "tcp"
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
    service = {
      target_group_arn = module.alb.target_groups["fete_ecs"].arn
      container_name   = local.container_name
      container_port   = local.api_port
    }
  }

  service_connect_configuration = {
    namespace = aws_service_discovery_http_namespace.this.arn
    service = {
      client_alias = {
        port     = local.p2p_port
        dns_name = local.container_name
      }
      port_name      = "p2p"
      discovery_name = local.container_name
    }
  }

  subnet_ids = module.vpc.private_subnets

  security_group_rules = {
    alb_http_ingress = {
      type                     = "ingress"
      from_port                = local.api_port
      to_port                  = local.api_port
      protocol                 = "TCP"
      description              = "Service port"
      source_security_group_id = module.alb.security_group_id
    }
  }
}
