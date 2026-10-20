variable "resource_group" {
  description = "Resource group name for all lab resources."
  type        = string
  default     = "wallet-lab-rg"
}

variable "location" {
  description = "Azure region. southeastasia (Bangkok) keeps latencies to the lab host low."
  type        = string
  default     = "southeastasia"
}

variable "acr_name" {
  description = "Azure Container Registry name (globally unique)."
  type        = string
  default     = "walletlab"
}

variable "acr_push_principal_id" {
  description = "Object ID of the ADO pipeline's Service Principal (the ARM service connection `wallet-azure`). It gets AcrPush on the registry."
  type        = string
}

variable "aks_name" {
  description = "AKS cluster name."
  type        = string
  default     = "wallet-aks"
}
