terraform {
  required_version = ">= 1.5"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 3.60"
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
  admin_user_enabled  = true
}

# The ARM service connection SP (created in ADO) needs ACR Contributor so
# `az acr login` + `docker push` from the pipeline works.
resource "azurerm_role_assignment" "acr_push" {
  scope                = azurerm_container_registry.wallet.id
  role_definition_name = "AcrPush"
  principal_id         = var.acr_push_principal_id
}

# ---------------------------------------------------------------------------
# AKS cluster
# ---------------------------------------------------------------------------
resource "azurerm_kubernetes_cluster" "wallet" {
  name                = var.aks_name
  resource_group_name = azurerm_resource_group.wallet.name
  location            = azurerm_resource_group.wallet.location
  kubernetes_version  = "1.33"
  dns_prefix          = "wallet"
  # Public API server: the ADO hosted agent needs to reach it.
  # For private, wire an outbound NAT / ADO self-hosted runner with a VNet.
  identity {
    type = "SystemAssigned"
  }
  default_node_pool {
    name       = "system"
    node_count = 1
    vm_size    = "Standard_D2s_v3"
  }
  # Keep stateful volumes (postgres, prometheus, redis) on managed premium
  # disks — also what `fsGroup: 70` needs (CSI honors fsGroup on these).
  azure_policy_enabled = true
}

# Allow the AKS control plane to reach the ACR without any credentials
# (image pulls in the pipeline + inside the cluster will work).
resource "azurerm_role_assignment" "aks_pull" {
  scope                = azurerm_container_registry.wallet.id
  role_definition_name = "AcrPull"
  principal_id         = azurerm_kubernetes_cluster.wallet.identity[0].principal_id
}

output "acr_login_server" {
  value = azurerm_container_registry.wallet.login_server
}

output "aks_fqdn" {
  value = azurerm_kubernetes_cluster.wallet.kubernetes_service_endpoint
}

output "subscription_id" {
  value = data.azurerm_client_config.current.subscription_id
}

data "azurerm_client_config" "current" {}
