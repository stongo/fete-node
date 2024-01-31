module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.2.0"
  azs                                = ["us-east-2a", "us-east-2b", "us-east-2c"]
  cidr                               = local.cidr
  public_subnets                     = local.public_subnets
  database_subnets                   = local.private_subnets
  enable_dns_support                 = true
  enable_dns_hostnames               = true
  map_public_ip_on_launch            = true
  enable_nat_gateway = true
  single_nat_gateway = true
}
