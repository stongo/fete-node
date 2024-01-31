locals {
  region         = "us-east-2"
  name           = "fete-network"
  container_name = "fete-node"

  vpc_cidr = "10.0.0.0/16"
  azs      = ["us-east-2a", "us-east-2b", "us-east-2c"]
  api_port = 5000
  p2p_port = 4001

  image = "stongo/fete-node:master"
  tags = {
    Name = local.name
    Role = "demo"
  }
}
