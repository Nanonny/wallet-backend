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

variable "aks_name" {
  description = "AKS cluster name."
  type        = string
  default     = "wallet-aks"
}

# This consumer subscription does not allow the plain Dv3 SKUs (Standard_D2s_v3)
# in southeastasia — the allowed D-family SKU is the SSD variant DC2ds_v3
# (same 2 vCPU / 8 GB). If provisioning is still blocked by quota, try the
# next candidates from `az vm list-skus --location southeastasia`.
variable "node_vm_size" {
  description = "AKS node VM size allowed by this subscription/region."
  type        = string
  default     = "Standard_DC2ds_v3"
}
