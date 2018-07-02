package azurerm

import (
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/services/servicebus/mgmt/2017-04-01/servicebus"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func resourceArmServiceBusTopic() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmServiceBusTopicCreate,
		Read:   resourceArmServiceBusTopicRead,
		Update: resourceArmServiceBusTopicCreate,
		Delete: resourceArmServiceBusTopicDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"namespace_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"location": deprecatedLocationSchema(),

			"resource_group_name": resourceGroupNameSchema(),

			"status": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  string(servicebus.Active),
				ValidateFunc: validation.StringInSlice([]string{
					string(servicebus.Active),
					string(servicebus.Disabled),
				}, true),
				DiffSuppressFunc: ignoreCaseDiffSuppressFunc,
			},

			"auto_delete_on_idle": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"default_message_ttl": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"duplicate_detection_history_time_window": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"enable_batched_operations": {
				Type:     schema.TypeBool,
				Optional: true,
			},

			"enable_express": {
				Type:     schema.TypeBool,
				Optional: true,
			},

			"enable_partitioning": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
			},

			"max_size_in_megabytes": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},

			"requires_duplicate_detection": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
			},

			"support_ordering": {
				Type:     schema.TypeBool,
				Optional: true,
			},

			// TODO: remove in the next major version
			"enable_filtering_messages_before_publishing": {
				Type:       schema.TypeBool,
				Optional:   true,
				Deprecated: "This field has been removed by Azure",
			},
		},
	}
}

func resourceArmServiceBusTopicCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).serviceBusTopicsClient
	ctx := meta.(*ArmClient).StopContext
	log.Printf("[INFO] preparing arguments for Azure ServiceBus Topic creation.")

	name := d.Get("name").(string)
	namespaceName := d.Get("namespace_name").(string)
	resourceGroup := d.Get("resource_group_name").(string)
	status := d.Get("status").(string)

	enableBatchedOps := d.Get("enable_batched_operations").(bool)
	enableExpress := d.Get("enable_express").(bool)
	enablePartitioning := d.Get("enable_partitioning").(bool)
	maxSize := int32(d.Get("max_size_in_megabytes").(int))
	requiresDuplicateDetection := d.Get("requires_duplicate_detection").(bool)
	supportOrdering := d.Get("support_ordering").(bool)

	parameters := servicebus.SBTopic{
		Name: &name,
		SBTopicProperties: &servicebus.SBTopicProperties{
			Status:                     servicebus.EntityStatus(status),
			EnableBatchedOperations:    utils.Bool(enableBatchedOps),
			EnableExpress:              utils.Bool(enableExpress),
			EnablePartitioning:         utils.Bool(enablePartitioning),
			MaxSizeInMegabytes:         utils.Int32(maxSize),
			RequiresDuplicateDetection: utils.Bool(requiresDuplicateDetection),
			SupportOrdering:            utils.Bool(supportOrdering),
		},
	}

	if autoDeleteOnIdle := d.Get("auto_delete_on_idle").(string); autoDeleteOnIdle != "" {
		parameters.SBTopicProperties.AutoDeleteOnIdle = utils.String(autoDeleteOnIdle)
	}

	if defaultTTL := d.Get("default_message_ttl").(string); defaultTTL != "" {
		parameters.SBTopicProperties.DefaultMessageTimeToLive = utils.String(defaultTTL)
	}

	if duplicateWindow := d.Get("duplicate_detection_history_time_window").(string); duplicateWindow != "" {
		parameters.SBTopicProperties.DuplicateDetectionHistoryTimeWindow = utils.String(duplicateWindow)
	}

	_, err := client.CreateOrUpdate(ctx, resourceGroup, namespaceName, name, parameters)
	if err != nil {
		return err
	}

	read, err := client.Get(ctx, resourceGroup, namespaceName, name)
	if err != nil {
		return err
	}
	if read.ID == nil {
		return fmt.Errorf("Cannot read ServiceBus Topic %s (resource group %s) ID", name, resourceGroup)
	}

	d.SetId(*read.ID)

	return resourceArmServiceBusTopicRead(d, meta)
}

func resourceArmServiceBusTopicRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).serviceBusTopicsClient
	ctx := meta.(*ArmClient).StopContext

	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resourceGroup := id.ResourceGroup
	namespaceName := id.Path["namespaces"]
	name := id.Path["topics"]

	resp, err := client.Get(ctx, resourceGroup, namespaceName, name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("Error making Read request on Azure ServiceBus Topic %s: %+v", name, err)
	}

	d.Set("name", resp.Name)
	d.Set("resource_group_name", resourceGroup)
	d.Set("namespace_name", namespaceName)

	if props := resp.SBTopicProperties; props != nil {
		d.Set("status", string(props.Status))
		d.Set("auto_delete_on_idle", props.AutoDeleteOnIdle)
		d.Set("default_message_ttl", props.DefaultMessageTimeToLive)

		if window := props.DuplicateDetectionHistoryTimeWindow; window != nil && *window != "" {
			d.Set("duplicate_detection_history_time_window", *window)
		}

		d.Set("enable_batched_operations", props.EnableBatchedOperations)
		d.Set("enable_express", props.EnableExpress)
		d.Set("enable_partitioning", props.EnablePartitioning)
		d.Set("requires_duplicate_detection", props.RequiresDuplicateDetection)
		d.Set("support_ordering", props.SupportOrdering)

		if maxSizeMB := props.MaxSizeInMegabytes; maxSizeMB != nil {
			maxSize := int(*props.MaxSizeInMegabytes)

			// if the topic is in a premium namespace and partitioning is enabled then the
			// max size returned by the API will be 16 times greater than the value set
			if partitioning := props.EnablePartitioning; partitioning != nil && *partitioning {
				namespacesClient := meta.(*ArmClient).serviceBusNamespacesClient
				namespace, err := namespacesClient.Get(ctx, resourceGroup, namespaceName)
				if err != nil {
					return err
				}

				if namespace.Sku.Name != servicebus.Premium {
					const partitionCount = 16
					maxSize = int(*props.MaxSizeInMegabytes / partitionCount)
				}
			}

			d.Set("max_size_in_megabytes", maxSize)
		}
	}

	return nil
}

func resourceArmServiceBusTopicDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).serviceBusTopicsClient
	ctx := meta.(*ArmClient).StopContext

	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resourceGroup := id.ResourceGroup
	namespaceName := id.Path["namespaces"]
	name := id.Path["topics"]

	resp, err := client.Delete(ctx, resourceGroup, namespaceName, name)
	if err != nil {
		if !utils.ResponseWasNotFound(resp) {
			return err
		}
	}

	return nil
}
