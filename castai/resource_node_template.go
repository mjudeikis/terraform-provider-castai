package castai

import (
	"context"
	"fmt"
	"github.com/castai/terraform-provider-castai/castai/sdk"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/samber/lo"
	"log"
	"regexp"
	"strings"
	"time"
)

const (
	FieldNodeTemplateName                      = "name"
	FieldNodeTemplateConfigurationId           = "configuration_id"
	FieldNodeTemplateShouldTaint               = "should_taint"
	FieldNodeTemplateRebalancingConfigMinNodes = "rebalancing_config_min_nodes"
	FieldNodeTemplateCustomLabel               = "custom_label"
	FieldNodeTemplateCustomLabels              = "custom_labels"
	FieldNodeTemplateCustomTaints              = "custom_taints"
	FieldNodeTemplateCustomInstancesEnabled    = "custom_instances_enabled"
	FieldNodeTemplateConstraints               = "constraints"
)

const (
	ArchAMD64 = "amd64"
	ArchARM64 = "arm64"
)

func resourceNodeTemplate() *schema.Resource {
	supportedArchitectures := []string{ArchAMD64, ArchARM64}

	return &schema.Resource{
		CreateContext: resourceNodeTemplateCreate,
		ReadContext:   resourceNodeTemplateRead,
		DeleteContext: resourceNodeTemplateDelete,
		UpdateContext: resourceNodeTemplateUpdate,
		Importer: &schema.ResourceImporter{
			StateContext: nodeTemplateStateImporter,
		},
		Description: "CAST AI node template resource to manage node templates",

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(1 * time.Minute),
			Read:   schema.DefaultTimeout(1 * time.Minute),
			Update: schema.DefaultTimeout(1 * time.Minute),
			Delete: schema.DefaultTimeout(1 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			FieldClusterId: {
				Type:             schema.TypeString,
				Optional:         true,
				ValidateDiagFunc: validation.ToDiagFunc(validation.IsUUID),
				Description:      "CAST AI cluster id.",
			},
			FieldNodeTemplateName: {
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace),
				Description:      "Name of the node template.",
			},
			FieldNodeTemplateConfigurationId: {
				Type:             schema.TypeString,
				Optional:         true,
				ValidateDiagFunc: validation.ToDiagFunc(validation.IsUUID),
				Description:      "CAST AI node configuration id to be used for node template.",
			},
			FieldNodeTemplateShouldTaint: {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Marks whether the templated nodes will have a taint.",
			},
			FieldNodeTemplateConstraints: {
				Type:     schema.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"spot": {
							Type:        schema.TypeBool,
							Default:     false,
							Optional:    true,
							Description: "Spot instance constraint - true only spot, false only on-demand.",
						},
						"use_spot_fallbacks": {
							Type:        schema.TypeBool,
							Default:     false,
							Optional:    true,
							Description: "Spot instance fallback constraint - when true, on-demand instances will be created, when spots are unavailable.",
						},
						"fallback_restore_rate_seconds": {
							Type:        schema.TypeInt,
							Default:     0,
							Optional:    true,
							Description: "Fallback restore rate in seconds: defines how much time should pass before spot fallback should be attempted to be restored to real spot.",
						},
						"min_cpu": {
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "Min CPU cores per node.",
						},
						"max_cpu": {
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "Max CPU cores per node.",
						},
						"min_memory": {
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "Min Memory (Mib) per node.",
						},
						"max_memory": {
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "Max Memory (Mib) per node.",
						},
						"storage_optimized": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     false,
							Description: "Storage optimized instance constraint - will only pick storage optimized nodes if true",
						},
						"compute_optimized": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     false,
							Description: "Compute optimized instance constraint - will only pick compute optimized nodes if true.",
						},
						"instance_families": {
							Type:     schema.TypeList,
							MaxItems: 1,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"include": {
										Type:     schema.TypeList,
										Optional: true,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
										Description: "Instance families to exclude when filtering (includes all other families).",
									},
									"exclude": {
										Type:     schema.TypeList,
										Optional: true,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
										Description: "Instance families to include when filtering (excludes all other families).",
									},
								},
							},
						},
						"gpu": {
							Type:     schema.TypeList,
							MaxItems: 1,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"manufacturers": {
										Type:     schema.TypeList,
										Optional: true,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
										Description: "Manufacturers of the gpus to select - NVIDIA, AMD.",
									},
									"include_names": {
										Type:     schema.TypeList,
										Optional: true,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
										Description: "Instance families to include when filtering (excludes all other families).",
									},
									"exclude_names": {
										Type:     schema.TypeList,
										Optional: true,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
										Description: "Names of the GPUs to exclude.",
									},
									"min_count": {
										Type:        schema.TypeInt,
										Optional:    true,
										Description: "Min GPU count for the instance type to have.",
									},
									"max_count": {
										Type:        schema.TypeInt,
										Optional:    true,
										Description: "Max GPU count for the instance type to have.",
									},
								},
							},
						},
						"architectures": {
							Type:     schema.TypeList,
							MaxItems: 2,
							MinItems: 1,
							Optional: true,
							Computed: true,
							Elem: &schema.Schema{
								Type:             schema.TypeString,
								ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice(supportedArchitectures, false)),
							},
							DefaultFunc: func() (interface{}, error) {
								return []string{ArchAMD64}, nil
							},
							Description: fmt.Sprintf("List of acceptable instance CPU architectures, the default is %s. Allowed values: %s.", ArchAMD64, strings.Join(supportedArchitectures, ", ")),
						},
					},
				},
			},
			FieldNodeTemplateCustomLabel: {
				Type:     schema.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"key": {
							Required:         true,
							Type:             schema.TypeString,
							ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace),
							Description:      "Label key to be added to nodes created from this template.",
						},
						"value": {
							Required:         true,
							Type:             schema.TypeString,
							ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace),
							Description:      "Label value to be added to nodes created from this template.",
						},
					},
				},
				Description: "Custom label key/value to be added to nodes created from this template.",
				Deprecated:  "Remove the use of `custom_label` field. The custom labels should be set through the `custom_labels` field.",
			},
			FieldNodeTemplateCustomLabels: {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Custom labels to be added to nodes created from this template. " +
					"If the field `custom_label` is present, the value of `custom_labels` will be ignored.",
			},
			FieldNodeTemplateCustomTaints: {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"key": {
							Required:         true,
							Type:             schema.TypeString,
							ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace),
							Description:      "Key of a taint to be added to nodes created from this template.",
						},
						"value": {
							Required:         true,
							Type:             schema.TypeString,
							ValidateDiagFunc: validation.ToDiagFunc(validation.StringIsNotWhiteSpace),
							Description:      "Value of a taint to be added to nodes created from this template.",
						},
						"effect": {
							Optional: true,
							Type:     schema.TypeString,
							ValidateDiagFunc: validation.ToDiagFunc(
								validation.StringMatch(regexp.MustCompile("^NoSchedule$"), "effect must be NoSchedule"),
							),
							Description: "Effect of a taint to be added to nodes created from this template. The effect must always be NoSchedule.",
						},
					},
				},
				Description: "Custom taints to be added to the nodes created from this template. " +
					"`shouldTaint` has to be `true` in order to create/update the node template with custom taints. " +
					"If `shouldTaint` is `true`, but no custom taints are provided, the nodes will be tainted with the default node template taint.",
			},
			FieldNodeTemplateRebalancingConfigMinNodes: {
				Type:             schema.TypeInt,
				Optional:         true,
				Default:          0,
				ValidateDiagFunc: validation.ToDiagFunc(validation.IntAtLeast(0)),
				Description:      "Minimum nodes that will be kept when rebalancing nodes using this node template.",
			},
			FieldNodeTemplateCustomInstancesEnabled: {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
				Description: "Marks whether custom instances should be used when deciding which parts of inventory are available. " +
					"Custom instances are only supported in GCP.",
			},
		},
	}
}

func resourceNodeTemplateRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	log.Printf("[INFO] List Node Templates get call start")
	defer log.Printf("[INFO] List Node Templates get call end")

	clusterID := getClusterId(d)
	if clusterID == "" {
		log.Print("[INFO] ClusterId is missing. Will skip operation.")
		return nil
	}

	nodeTemplate, err := getNodeTemplateByName(ctx, d, meta, clusterID)
	if err != nil {
		return diag.FromErr(err)
	}
	if !d.IsNewResource() && nodeTemplate == nil {
		log.Printf("[WARN] Node template (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}
	if err := d.Set(FieldNodeTemplateName, nodeTemplate.Name); err != nil {
		return diag.FromErr(fmt.Errorf("setting name: %w", err))
	}
	if err := d.Set(FieldNodeTemplateConfigurationId, nodeTemplate.ConfigurationId); err != nil {
		return diag.FromErr(fmt.Errorf("setting configuration id: %w", err))
	}
	if err := d.Set(FieldNodeTemplateShouldTaint, nodeTemplate.ShouldTaint); err != nil {
		return diag.FromErr(fmt.Errorf("setting should taint: %w", err))
	}
	if nodeTemplate.RebalancingConfig != nil {
		if err := d.Set(FieldNodeTemplateRebalancingConfigMinNodes, nodeTemplate.RebalancingConfig.MinNodes); err != nil {
			return diag.FromErr(fmt.Errorf("setting configuration id: %w", err))
		}
	}
	if nodeTemplate.Constraints != nil {
		constraints, err := flattenConstraints(nodeTemplate.Constraints)
		if err != nil {
			return diag.FromErr(fmt.Errorf("flattening constraints: %w", err))
		}

		if err := d.Set(FieldNodeTemplateConstraints, constraints); err != nil {
			return diag.FromErr(fmt.Errorf("setting constraints: %w", err))
		}
	}
	if err := d.Set(FieldNodeTemplateCustomLabel, flattenCustomLabel(nodeTemplate.CustomLabel)); err != nil {
		return diag.FromErr(fmt.Errorf("setting custom label: %w", err))
	}
	if err := d.Set(FieldNodeTemplateCustomLabels, nodeTemplate.CustomLabels.AdditionalProperties); err != nil {
		return diag.FromErr(fmt.Errorf("setting custom labels: %w", err))
	}
	if err := d.Set(FieldNodeTemplateCustomTaints, flattenCustomTaints(nodeTemplate.CustomTaints)); err != nil {
		return diag.FromErr(fmt.Errorf("setting custom taints: %w", err))
	}
	if err := d.Set(FieldNodeTemplateCustomInstancesEnabled, lo.FromPtrOr(nodeTemplate.CustomInstancesEnabled, false)); err != nil {
		return diag.FromErr(fmt.Errorf("setting custom instances enabled: %w", err))
	}

	return nil
}

func flattenConstraints(c *sdk.NodetemplatesV1TemplateConstraints) ([]map[string]any, error) {
	if c == nil {
		return nil, nil
	}

	out := make(map[string]any)
	if c.Gpu != nil {
		out["gpu"] = flattenGpu(c.Gpu)
	}
	if c.InstanceFamilies != nil {
		out["instance_families"] = flattenInstanceFamilies(c.InstanceFamilies)
	}
	if c.ComputeOptimized != nil {
		out["compute_optimized"] = c.ComputeOptimized
	}
	if c.StorageOptimized != nil {
		out["storage_optimized"] = c.StorageOptimized
	}
	if c.Spot != nil {
		out["spot"] = c.Spot
	}

	if c.UseSpotFallbacks != nil {
		out["use_spot_fallbacks"] = c.UseSpotFallbacks
	}
	if c.FallbackRestoreRateSeconds != nil {
		out["fallback_restore_rate_seconds"] = c.FallbackRestoreRateSeconds
	}
	if c.MinMemory != nil {
		out["min_memory"] = c.MinMemory
	}
	if c.MaxMemory != nil {
		out["max_memory"] = c.MaxMemory
	}
	if c.MinCpu != nil {
		out["min_cpu"] = c.MinCpu
	}
	if c.MaxCpu != nil {
		out["max_cpu"] = c.MaxCpu
	}
	if c.Architectures != nil {
		out["architectures"] = lo.FromPtr(c.Architectures)
	}
	return []map[string]any{out}, nil
}

func flattenInstanceFamilies(families *sdk.NodetemplatesV1TemplateConstraintsInstanceFamilyConstraints) []map[string][]string {
	if families == nil {
		return nil
	}
	out := map[string][]string{}
	if families.Exclude != nil {
		out["exclude"] = lo.FromPtr(families.Exclude)
	}
	if families.Include != nil {
		out["include"] = lo.FromPtr(families.Include)
	}
	return []map[string][]string{out}
}

func flattenGpu(gpu *sdk.NodetemplatesV1TemplateConstraintsGPUConstraints) []map[string]any {
	if gpu == nil {
		return nil
	}
	out := map[string]any{}
	if gpu.ExcludeNames != nil {
		out["exclude_names"] = gpu.ExcludeNames
	}
	if gpu.IncludeNames != nil {
		out["include_names"] = gpu.IncludeNames
	}
	if gpu.Manufacturers != nil {
		out["manufacturers"] = gpu.Manufacturers
	}
	if gpu.MinCount != nil {
		out["min_count"] = gpu.MinCount
	}
	if gpu.MaxCount != nil {
		out["max_count"] = gpu.MaxCount
	}
	return []map[string]any{out}
}

func resourceNodeTemplateDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	client := meta.(*ProviderConfig).api
	clusterID := d.Get(FieldClusterID).(string)
	name := d.Get(FieldNodeTemplateName).(string)

	resp, err := client.NodeTemplatesAPIDeleteNodeTemplateWithResponse(ctx, clusterID, name)
	if checkErr := sdk.CheckOKResponse(resp, err); checkErr != nil {
		return diag.FromErr(checkErr)
	}

	return nil
}

func resourceNodeTemplateUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	if !d.HasChanges(
		FieldNodeTemplateName,
		FieldNodeTemplateShouldTaint,
		FieldNodeTemplateConfigurationId,
		FieldNodeTemplateRebalancingConfigMinNodes,
		FieldNodeTemplateCustomLabel,
		FieldNodeTemplateCustomLabels,
		FieldNodeTemplateCustomTaints,
		FieldNodeTemplateCustomInstancesEnabled,
		FieldNodeTemplateConstraints,
	) {
		log.Printf("[INFO] Nothing to update in node configuration")
		return nil
	}

	client := meta.(*ProviderConfig).api
	clusterID := d.Get(FieldClusterID).(string)
	name := d.Get(FieldNodeTemplateName).(string)

	req := sdk.NodeTemplatesAPIUpdateNodeTemplateJSONRequestBody{}
	if v, ok := d.GetOk(FieldNodeTemplateConfigurationId); ok {
		req.ConfigurationId = toPtr(v.(string))
	}

	if v, ok := d.Get(FieldNodeTemplateCustomLabel).([]any); ok && len(v) > 0 {
		req.CustomLabel = toCustomLabel(v[0].(map[string]any))
	}

	if req.CustomLabel == nil {
		if v, ok := d.Get(FieldNodeTemplateCustomLabels).(map[string]any); ok && len(v) > 0 {
			customLabels := map[string]string{}

			for k, v := range v {
				customLabels[k] = v.(string)
			}

			req.CustomLabels = &sdk.NodetemplatesV1UpdateNodeTemplate_CustomLabels{AdditionalProperties: customLabels}
		}
	}

	if v, _ := d.GetOk(FieldNodeTemplateShouldTaint); v != nil {
		req.ShouldTaint = toPtr(v.(bool))
	}

	if v, ok := d.Get(FieldNodeTemplateCustomTaints).([]any); ok && len(v) > 0 {
		ts := []map[string]any{}
		for _, val := range v {
			ts = append(ts, val.(map[string]any))
		}

		req.CustomTaints = toCustomTaintsWithoutEffect(ts)
	}

	if !(*req.ShouldTaint) && req.CustomTaints != nil && len(*req.CustomTaints) > 0 {
		return diag.FromErr(fmt.Errorf("shouldTaint must be true for the node template to get updated with custom taints"))
	}

	if v, _ := d.GetOk(FieldNodeTemplateRebalancingConfigMinNodes); v != nil {
		req.RebalancingConfig = &sdk.NodetemplatesV1RebalancingConfiguration{
			MinNodes: toPtr(int32(v.(int))),
		}
	}

	if v, ok := d.Get(FieldNodeTemplateConstraints).([]any); ok && len(v) > 0 {
		req.Constraints = toTemplateConstraints(v[0].(map[string]any))
	}

	if v, _ := d.GetOk(FieldNodeTemplateCustomInstancesEnabled); v != nil {
		req.CustomInstancesEnabled = lo.ToPtr(v.(bool))
	}

	resp, err := client.NodeTemplatesAPIUpdateNodeTemplateWithResponse(ctx, clusterID, name, req)
	if checkErr := sdk.CheckOKResponse(resp, err); checkErr != nil {
		return diag.FromErr(checkErr)
	}

	return resourceNodeTemplateRead(ctx, d, meta)
}

func resourceNodeTemplateCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	log.Printf("[INFO] Create Node Template post call start")
	defer log.Printf("[INFO] Create Node Template post call end")
	client := meta.(*ProviderConfig).api
	clusterID := d.Get(FieldClusterID).(string)
	req := sdk.NodeTemplatesAPICreateNodeTemplateJSONRequestBody{
		Name:            lo.ToPtr(d.Get(FieldNodeTemplateName).(string)),
		ConfigurationId: lo.ToPtr(d.Get(FieldNodeTemplateConfigurationId).(string)),
		ShouldTaint:     lo.ToPtr(d.Get(FieldNodeTemplateShouldTaint).(bool)),
	}

	if v, ok := d.Get(FieldNodeTemplateRebalancingConfigMinNodes).(int32); ok {
		req.RebalancingConfig = &sdk.NodetemplatesV1RebalancingConfiguration{
			MinNodes: lo.ToPtr(v),
		}
	}

	if v, ok := d.Get(FieldNodeTemplateCustomLabel).([]any); ok && len(v) > 0 {
		req.CustomLabel = toCustomLabel(v[0].(map[string]any))
	}

	if req.CustomLabel == nil {
		if v, ok := d.Get(FieldNodeTemplateCustomLabels).(map[string]any); ok && len(v) > 0 {
			customLabels := map[string]string{}

			for k, v := range v {
				customLabels[k] = v.(string)
			}

			req.CustomLabels = &sdk.NodetemplatesV1NewNodeTemplate_CustomLabels{AdditionalProperties: customLabels}
		}
	}

	if v, ok := d.Get(FieldNodeTemplateCustomTaints).([]any); ok && len(v) > 0 {
		ts := []map[string]any{}
		for _, val := range v {
			ts = append(ts, val.(map[string]any))
		}

		req.CustomTaints = toCustomTaintsWithoutEffect(ts)
	}

	if !(*req.ShouldTaint) && req.CustomTaints != nil && len(*req.CustomTaints) > 0 {
		return diag.FromErr(fmt.Errorf("shouldTaint must be true for the node template to get created with custom taints"))
	}

	if v, ok := d.Get(FieldNodeTemplateConstraints).([]any); ok && len(v) > 0 {
		req.Constraints = toTemplateConstraints(v[0].(map[string]any))
	}

	if v, _ := d.GetOk(FieldNodeTemplateCustomInstancesEnabled); v != nil {
		req.CustomInstancesEnabled = lo.ToPtr(v.(bool))
	}

	resp, err := client.NodeTemplatesAPICreateNodeTemplateWithResponse(ctx, clusterID, req)
	if checkErr := sdk.CheckOKResponse(resp, err); checkErr != nil {
		return diag.FromErr(checkErr)
	}

	d.SetId(lo.FromPtr(resp.JSON200.Name))

	return resourceNodeTemplateRead(ctx, d, meta)
}

func getNodeTemplateByName(ctx context.Context, data *schema.ResourceData, meta any, clusterID sdk.ClusterId) (*sdk.NodetemplatesV1NodeTemplate, error) {
	client := meta.(*ProviderConfig).api
	nodeTemplateName := data.Id()

	log.Printf("[INFO] Getting current node templates")
	resp, err := client.NodeTemplatesAPIListNodeTemplatesWithResponse(ctx, clusterID)
	notFound := fmt.Errorf("node templates for cluster %q not found at CAST AI", clusterID)
	if err != nil {
		return nil, err
	}

	templates := resp.JSON200

	if templates == nil {
		return nil, notFound
	}

	if err != nil {
		log.Printf("[WARN] Getting current node template: %v", err)
		return nil, fmt.Errorf("failed to get current node template from API: %v", err)
	}

	t, ok := lo.Find[sdk.NodetemplatesV1NodeTemplateListItem](lo.FromPtr(templates.Items), func(t sdk.NodetemplatesV1NodeTemplateListItem) bool {
		return lo.FromPtr(t.Template.Name) == nodeTemplateName
	})

	if !ok {
		return nil, fmt.Errorf("failed to find node template with name: %v", nodeTemplateName)
	}

	if err != nil {
		log.Printf("[WARN] Failed merging node template changes: %v", err)
		return nil, fmt.Errorf("failed to merge node template changes: %v", err)
	}

	return t.Template, nil
}

func nodeTemplateStateImporter(ctx context.Context, d *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
	ids := strings.Split(d.Id(), "/")
	if len(ids) != 2 || ids[0] == "" || ids[1] == "" {
		return nil, fmt.Errorf("expected import id with format: <cluster_id>/<node_template name or id>, got: %q", d.Id())
	}

	clusterID, id := ids[0], ids[1]
	if err := d.Set(FieldClusterID, clusterID); err != nil {
		return nil, fmt.Errorf("setting cluster id: %w", err)
	}
	d.SetId(id)

	// Return if node config ID provided.
	if _, err := uuid.Parse(id); err == nil {
		return []*schema.ResourceData{d}, nil
	}

	// Find node templates
	client := meta.(*ProviderConfig).api
	resp, err := client.NodeTemplatesAPIListNodeTemplatesWithResponse(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	if resp.JSON200 != nil {
		for _, cfg := range *resp.JSON200.Items {
			name := toString(cfg.Template.Name)
			if name == id {
				d.SetId(name)
				return []*schema.ResourceData{d}, nil
			}
		}
	}

	return nil, fmt.Errorf("failed to find node template with the following name: %v", id)
}

func toCustomLabel(obj map[string]any) *sdk.NodetemplatesV1Label {
	if obj == nil {
		return nil
	}

	out := &sdk.NodetemplatesV1Label{}
	if v, ok := obj["key"]; ok && v != "" {
		out.Key = toPtr(v.(string))
	}
	if v, ok := obj["value"]; ok && v != "" {
		out.Value = toPtr(v.(string))
	}

	return out
}

func toCustomTaintsWithoutEffect(objs []map[string]any) *[]sdk.NodetemplatesV1TaintWithoutEffect {
	if len(objs) == 0 {
		return nil
	}

	out := &[]sdk.NodetemplatesV1TaintWithoutEffect{}

	for _, taint := range objs {
		t := sdk.NodetemplatesV1TaintWithoutEffect{}

		if v, ok := taint["key"]; ok && v != "" {
			t.Key = toPtr(v.(string))
		}
		if v, ok := taint["value"]; ok && v != "" {
			t.Value = toPtr(v.(string))
		}

		*out = append(*out, t)
	}

	return out
}

func flattenCustomLabel(label *sdk.NodetemplatesV1Label) []map[string]string {
	if label == nil {
		return nil
	}

	m := map[string]string{}
	if v := label.Key; v != nil {
		m["key"] = toString(v)
	}
	if v := label.Value; v != nil {
		m["value"] = toString(v)
	}
	return []map[string]string{m}
}

func flattenCustomTaints(taints *[]sdk.NodetemplatesV1Taint) []map[string]string {
	if taints == nil {
		return nil
	}

	var ts []map[string]string
	for _, taint := range *taints {
		t := map[string]string{}
		if k := taint.Key; k != nil {
			t["key"] = toString(k)
		}
		if v := taint.Value; v != nil {
			t["value"] = toString(v)
		}
		if e := taint.Effect; e != nil {
			t["effect"] = toString(e)
		}

		ts = append(ts, t)
	}

	return ts
}

func toTemplateConstraints(obj map[string]any) *sdk.NodetemplatesV1TemplateConstraints {
	if obj == nil {
		return nil
	}

	out := &sdk.NodetemplatesV1TemplateConstraints{}
	if v, ok := obj["compute_optimized"].(bool); ok {
		out.ComputeOptimized = toPtr(v)
	}
	if v, ok := obj["fallback_restore_rate_seconds"].(int); ok {
		out.FallbackRestoreRateSeconds = toPtr(int32(v))
	}
	if v, ok := obj["gpu"].([]any); ok && len(v) > 0 {
		out.Gpu = toTemplateConstraintsGpuConstraints(v[0].(map[string]any))
	}
	if v, ok := obj["instance_families"].([]any); ok && len(v) > 0 {
		out.InstanceFamilies = toTemplateConstraintsInstanceFamilies(v[0].(map[string]any))
	}
	if v, ok := obj["max_cpu"].(int); ok && v != 0 {
		out.MaxCpu = toPtr(int32(v))
	}
	if v, ok := obj["max_memory"].(int); ok && v != 0 {
		out.MaxMemory = toPtr(int32(v))
	}
	if v, ok := obj["min_cpu"].(int); ok {
		out.MinCpu = toPtr(int32(v))
	}
	if v, ok := obj["min_memory"].(int); ok {
		out.MinMemory = toPtr(int32(v))
	}
	if v, ok := obj["spot"].(bool); ok {
		out.Spot = toPtr(v)
	}
	if v, ok := obj["storage_optimized"].(bool); ok {
		out.StorageOptimized = toPtr(v)
	}
	if v, ok := obj["use_spot_fallbacks"].(bool); ok {
		out.UseSpotFallbacks = toPtr(v)
	}
	if v, ok := obj["architectures"].([]any); ok {
		out.Architectures = toPtr(toStringList(v))
	}

	return out
}

func toTemplateConstraintsInstanceFamilies(o map[string]any) *sdk.NodetemplatesV1TemplateConstraintsInstanceFamilyConstraints {
	if o == nil {
		return nil
	}

	out := &sdk.NodetemplatesV1TemplateConstraintsInstanceFamilyConstraints{}
	if v, ok := o["exclude"].([]any); ok {
		out.Exclude = toPtr(toStringList(v))
	}
	if v, ok := o["include"].([]any); ok {
		out.Include = toPtr(toStringList(v))
	}
	return out
}

func toTemplateConstraintsGpuConstraints(o map[string]any) *sdk.NodetemplatesV1TemplateConstraintsGPUConstraints {
	if o == nil {
		return nil
	}

	out := &sdk.NodetemplatesV1TemplateConstraintsGPUConstraints{}
	if v, ok := o["manufacturers"].([]any); ok {
		out.Manufacturers = toPtr(toStringList(v))
	}

	if v, ok := o["exclude_names"].([]any); ok {
		out.ExcludeNames = toPtr(toStringList(v))
	}
	if v, ok := o["include_names"].([]any); ok {
		out.IncludeNames = toPtr(toStringList(v))
	}

	if v, ok := o["min_count"].(int); ok {
		out.MinCount = toPtr(int32(v))
	}
	if v, ok := o["max_count"].(int); ok && v != 0 {
		out.MaxCount = toPtr(int32(v))
	}

	return out
}
