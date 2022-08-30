package castai

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccResourceNodeConfiguration_basic(t *testing.T) {
	rName := fmt.Sprintf("%v-node-config-%v", ResourcePrefix, acctest.RandString(8))
	resourceName := "castai_node_configuration.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		CheckDestroy:      testAccCheckNodeConfigurationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccNodeConfigurationConfig(rName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttr(resourceName, "disk_cpu_ratio", "35"),
					resource.TestCheckResourceAttr(resourceName, "image", ""),
					resource.TestCheckResourceAttr(resourceName, "ssh_public_key", ""),
					resource.TestCheckResourceAttr(resourceName, "subnets.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.env", "development"),
					resource.TestCheckResourceAttrSet(resourceName, "eks.0.instance_profile_arn"),
					resource.TestCheckResourceAttr(resourceName, "eks.0.dns_cluster_ip", "10.100.0.10"),
					resource.TestCheckResourceAttr(resourceName, "eks.0.security_groups.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "eks.0.key_pair_id", ""),
					resource.TestCheckResourceAttr(resourceName, "aks.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "kops.#", "0"),
				),
			},
			{
				Config: testAccNodeConfigurationUpdated(rName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "disk_cpu_ratio", "25"),
					resource.TestCheckResourceAttr(resourceName, "image", "amazon-eks-node-1.23-v20220824"),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "eks.0.dns_cluster_ip", ""),
					resource.TestCheckResourceAttr(resourceName, "eks.0.security_groups.#", "1"),
				),
			},
		},
		ExternalProviders: map[string]resource.ExternalProvider{
			"aws": {
				Source:            "hashicorp/aws",
				VersionConstraint: "~> 4.0",
			},
		},
	})
}

func testAccNodeConfigurationConfig(rName string) string {
	return ConfigCompose(testAccClusterConfig(rName), fmt.Sprintf(`
resource "castai_node_configuration" "test" {
  name   		  = %[1]q
  cluster_id      = castai_eks_cluster.test.id
  disk_cpu_ratio  = 35
  subnets   	  = aws_subnet.test[*].id
  tags = {
    env = "development"
  }
  eks {
	instance_profile_arn = aws_iam_instance_profile.test.arn
    dns_cluster_ip       = "10.100.0.10"
	security_groups      = [aws_security_group.test.id]
  }
}

resource "castai_node_configuration_default" "test" {
  cluster_id       = castai_eks_cluster.test.id
  configuration_id = castai_node_configuration.test.id
}
`, rName))
}

func testAccNodeConfigurationUpdated(rName string) string {
	return ConfigCompose(testAccClusterConfig(rName), fmt.Sprintf(`
resource "castai_node_configuration" "test" {
  name   		  = %[1]q
  cluster_id      = castai_eks_cluster.test.id
  subnets   	  = aws_subnet.test[*].id
  image           = "amazon-eks-node-1.23-v20220824" 
  eks {
	instance_profile_arn = aws_iam_instance_profile.test.arn
    security_groups      = [aws_security_group.test.id]
  }
}`, rName))
}

func testAccClusterConfig(rName string) string {
	return ConfigCompose(testAccAWSConfig(rName), fmt.Sprintf(`
data "castai_eks_clusterid" "test" {
  account_id   = data.aws_caller_identity.current.account_id
  region       = "eu-central-1"
  cluster_name = %[1]q
}

data "castai_eks_user_arn" "test" {
  cluster_id = data.castai_eks_clusterid.test.id
}

resource "castai_eks_cluster" "test" {
  account_id      = data.aws_caller_identity.current.account_id
  region          = "eu-central-1"
  name            = %[1]q
  assume_role_arn = aws_iam_role.test.arn
}
`, rName))
}

func testAccAWSConfig(rName string) string {
	return fmt.Sprintf(`
provider "aws" {
  region = "eu-central-1"
}

data "aws_caller_identity" "current" {}

resource "aws_vpc" "test" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true
  tags = {
    name = %[1]q
  }
}

resource "aws_subnet" "test" {
  count = 2
  cidr_block              = cidrsubnet(aws_vpc.test.cidr_block, 8, count.index)
  map_public_ip_on_launch = true
  vpc_id                  = aws_vpc.test.id
  tags = {
    Name = %[1]q
  }
}

resource "aws_security_group" "test" {
  name        = %[1]q
  vpc_id      = aws_vpc.test.id

  ingress {
    from_port        = 443
    to_port          = 443
    protocol         = "tcp"
    cidr_blocks      = [aws_vpc.test.cidr_block]
  }

  egress {
    from_port        = 0
    to_port          = 0
    protocol         = "-1"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
  }
}

resource "aws_iam_role" "test" {
  name               = %[1]q
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [
      {
        Action    = "sts:AssumeRole"
        Effect    = "Allow"
        Principal = {
          AWS = data.castai_eks_user_arn.test.arn
        }
      },
    ]
  })
}

resource "aws_iam_role_policy_attachment" "test" {
  role       = aws_iam_role.test.name
  policy_arn = "arn:aws:iam::aws:policy/AdministratorAccess"
}

resource "aws_iam_instance_profile" "test" {
  name = "%[1]v-node-profile"
  role = aws_iam_role.node.name
}

resource "aws_iam_role" "node" {
  name = "%[1]v-node"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [
      {
        Action    = "sts:AssumeRole"
        Effect    = "Allow"
        Principal = {
          "Service": "ec2.amazonaws.com"
        }
      },
    ]
  })
}
`, rName)
}

func testAccCheckNodeConfigurationDestroy(s *terraform.State) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := testAccProvider.Meta().(*ProviderConfig).api
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "castai_node_configuration" {
			continue
		}

		id := rs.Primary.ID
		clusterID := rs.Primary.Attributes["cluster_id"]
		response, err := client.NodeConfigurationAPIGetConfigurationWithResponse(ctx, clusterID, id)
		if err != nil {
			return err
		}
		if response.StatusCode() == http.StatusNotFound {
			return nil
		}
		if *response.JSON200.Default {
			// Default node config can't be deleted.
			return nil
		}

		return fmt.Errorf("node configuration %s still exists", rs.Primary.ID)
	}

	return nil
}
