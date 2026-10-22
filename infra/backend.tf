# Terraform state backend
#
# The ADO Terraform task uses `use_backend: local` (see azure-pipelines.yml).
# For a lab environment local state under `infra/terraform.tfstate` is fine
# (the ACR admin password is not stored in state — only ids/names).
#
# For a production-grade setup switch to an Azure Storage backend:
#
#   backend "azurerm" {
#     resource_group_name = "wallet-lab-rg"
#     storage_account_name = "walletlabstate"
#     container_name       = "terraform-state"
#     key                  = "wallet-lab.tfstate"
#   }
#
terraform {
  backend "local" {
    workspace_dir = "infra"
  }
}
