terraform {
  required_version = ">= 1.5"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      # Pinned on purpose: azurerm 4.x changed the schema (dropped
      # `admin_user_enabled` on container registries and
      # `default_node_pool` on AKS). 3.x is the stable, known-good schema
      # for this module, and pinning keeps `terraform init` deterministic.
      version = "3.117.1"
    }
  }
}

provider "azurerm" {
  # Credentials come from the Azure DevOps variable group `wallet-azure-vars`
  # (or the ARM service connection `wallet-azure`) — the ADO Terraform task
  # injects AZURE_CLIENT_ID / AZURE_CLIENT_SECRET / AZURE_TENANT_ID /
  # AZURE_SUBSCRIPTION_ID for the azurerm provider's `features` block.
  features {}
}

# ---------------------------------------------------------------------------
# Resource group
# ---------------------------------------------------------------------------
resource "azurerm_resource_group" "wallet" {
  name     = var.resource_group
  location = var.location
}

# ---------------------------------------------------------------------------
# Azure Container Registry (Basic SKU)
# ---------------------------------------------------------------------------
resource "azurerm_container_registry" "wallet" {
  name                = var.acr_name
  resource_group_name = azurerm_resource_group.wallet.name
  location            = azurerm_resource_group.wallet.location
  sku                 = "Basic"
  admin_enabled       = true
}

# ---------------------------------------------------------------------------
# AKS cluster
# ---------------------------------------------------------------------------
resource "azurerm_kubernetes_cluster" "wallet" {
  name                = var.aks_name
  resource_group_name = azurerm_resource_group.wallet.name
  location            = azurerm_resource_group.wallet.location
  # 1.37 is the latest version in this region whose SupportPlan includes
  # "KubernetesOfficial" (standard support, no extra cost). Older versions
  # (1.3x) have moved to "AKSLongTermSupport" (requires a license/paid add-on).
  kubernetes_version  = "1.37.0"
  dns_prefix          = "wallet"
  # Public API server: the ADO hosted agent needs to reach it.
  # For private, wire an outbound NAT / ADO self-hosted runner with a VNet.
  identity {
    type = "SystemAssigned"
  }
  default_node_pool {
    name       = "system"
    node_count = 1
    vm_size    = var.node_vm_size
  }
  # Keep stateful volumes (postgres, prometheus, redis) on managed premium
  # disks — also what `fsGroup: 70` needs (CSI honors fsGroup on these).
  azure_policy_enabled = true
}

# NOTE: no azurerm_role_assignment here. This lab uses a consumer subscription
# which does not permit `Microsoft.Authorization/roleAssignments/write`, so
# role-based ACR access (AcrPush/AcrPull) is not possible. Instead:
#   - the pipeline pushes with the ACR admin user (admin_enabled = true),
#   - the pipeline seeds a docker-registry imagePullSecret into the cluster.

output "acr_login_server" {
  value = azurerm_container_registry.wallet.login_server
}

output "aks_fqdn" {
  value = azurerm_kubernetes_cluster.wallet.fqdn
}

output "subscription_id" {
  value = data.azurerm_client_config.current.subscription_id
}

data "azurerm_client_config" "current" {}
