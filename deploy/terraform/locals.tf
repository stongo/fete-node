locals {
  region = "us-east-2"
  name   = "fete-network"

  vpc_cidr = "10.0.0.0/16"
  public_subnets  = ["10.0.2.0/24"]
  private_subnets = ["10.0.128.0/24", "10.0.129.0/24"]
  azs      = ["us-east-2a", "us-east-2b", "us-east-2c"] 
  api_port = 5000
  p2p_port = 4001

  image = "stongo/fete-node:latest"
  tags = {
    Name       = local.name
    Role = "demo"
  }
}
