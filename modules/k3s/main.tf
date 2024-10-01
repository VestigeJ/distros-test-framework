module "master" {
   source               ="./master"
   aws_ami              = var.aws_ami
   aws_user             = var.aws_user
   key_name             = var.key_name
   no_of_server_nodes   = var.no_of_server_nodes
   k3s_version          = var.k3s_version
   install_mode         = var.install_mode
   region               = var.region
   vpc_id               = var.vpc_id
   subnets              = var.subnets
   qa_space             = var.qa_space
   ec2_instance_class   = var.ec2_instance_class
   access_key           = var.access_key
   datastore_type       = var.datastore_type
   server_flags         = var.server_flags
   availability_zone    = var.availability_zone
   sg_id                = var.sg_id
   volume_size          = var.volume_size
   resource_name        = var.resource_name
   node_os              = var.node_os
   username             = var.username
   password             = var.password
   db_username          = var.db_username
   db_password          = var.db_password
   db_group_name        = var.db_group_name
   external_db          = var.external_db
   instance_class       = var.instance_class
   external_db_version  = var.external_db_version
   engine_mode          = var.engine_mode
   environment          = var.environment
   create_lb            = var.create_lb
   k3s_channel          = var.k3s_channel
   enable_public_ip     = var.enable_public_ip
   enable_ipv6          = var.enable_ipv6

   # Split roles
   all_role_nodes       = var.no_of_server_nodes
   etcd_only_nodes      = var.etcd_only_nodes
   etcd_cp_nodes        = var.etcd_cp_nodes
   etcd_worker_nodes    = var.etcd_worker_nodes
   cp_only_nodes        = var.cp_only_nodes
   cp_worker_nodes      = var.cp_worker_nodes
   split_roles          = var.split_roles
   role_order           = var.role_order
   create_eip            = var.create_eip
}
module "worker" {
   source               = "./worker"
   dependency           = module.master
   aws_ami              = var.aws_ami
   aws_user             = var.aws_user
   key_name             = var.key_name
   no_of_worker_nodes   = var.no_of_worker_nodes
   k3s_version          = var.k3s_version
   install_mode         = var.install_mode
   region               = var.region
   vpc_id               = var.vpc_id
   subnets              = var.subnets
   ec2_instance_class   = var.ec2_instance_class
   access_key           = var.access_key
   worker_flags         = var.worker_flags
   availability_zone    = var.availability_zone
   sg_id                = var.sg_id
   volume_size          = var.volume_size
   resource_name        = var.resource_name
   node_os              = var.node_os
   username             = var.username
   password             = var.password
   k3s_channel          = var.k3s_channel
   enable_public_ip     = var.enable_public_ip
   enable_ipv6          = var.enable_ipv6
   create_eip            = var.create_eip
}

module "bastion" {
   source               = "../bastion"
   aws_ami              = var.aws_ami
   aws_user             = var.aws_user
   ec2_instance_class   = var.ec2_instance_class
   region               = var.region
   vpc_id               = var.vpc_id
   bastion_subnets      = var.bastion_subnets
   availability_zone    = var.availability_zone
   sg_id                = var.sg_id
   volume_size          = var.volume_size
   enable_public_ip     = var.enable_public_ip
   enable_ipv6          = var.enable_ipv6
   key_name             = var.key_name
   access_key           = var.access_key
   resource_name        = var.resource_name
   no_of_bastion_nodes  = var.no_of_bastion_nodes
}

