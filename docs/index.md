---
page_title: "CAST AI Provider"
description: |-
  Use CAST AI provider to onboard the cluster and manage resources supported by CAST AI.
---

# CAST AI Provider

CAST AI provider can be used to onboard your cluster and manage resources supported by CAST AI.

-> **Note** To use the provider, an API token first must be generated for your account at https://console.cast.ai/

## Provider configuration

Terraform 0.13 and later:

```terraform
terraform {
  required_providers {
    castai = {
      source  = "castai/castai"
      version = "2.0.0"
    }
  }
}

# Configure the provider
provider "castai" {
  api_token = "my-castai-api-token"
}
```

## Example Usage

```terraform
# Connect EKS cluster to CAST AI in read-only mode.

# Configure Data sources and providers required for CAST AI connection.
data "aws_caller_identity" "current" {}

provider "castai" {
  api_token = var.castai_api_token
}

provider "helm" {
  kubernetes {
    host                   = module.eks.cluster_endpoint
    cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)
      exec {
        api_version = "client.authentication.k8s.io/v1beta1"
        command     = "aws"
        # This requires the awscli to be installed locally where Terraform is executed.
        args = ["eks", "get-token", "--cluster-name", module.eks.cluster_name]
      }
  }
}


# Configure EKS cluster connection to CAST AI in read-only mode.
resource "castai_eks_cluster" "this" {
  account_id = data.aws_caller_identity.current.account_id
  region     = var.cluster_region
  name       = var.cluster_name
}

resource "helm_release" "castai_agent" {
  name             = "castai-agent"
  repository       = "https://castai.github.io/helm-charts"
  chart            = "castai-agent"
  namespace        = "castai-agent"
  create_namespace = true
  cleanup_on_fail  = true

  set {
    name  = "provider"
    value = "eks"
  }
  set_sensitive {
    name  = "apiKey"
    value = castai_eks_cluster.this.cluster_token
  }

  # Required until https://github.com/castai/helm-charts/issues/135 is fixed.
  set {
    name  = "createNamespace"
    value = "false"
  }
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `api_token` (String) The token used to connect to CAST AI API.

### Optional

- `api_url` (String) CAST.AI API url.