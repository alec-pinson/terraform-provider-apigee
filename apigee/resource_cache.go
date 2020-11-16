package apigee

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-http-utils/headers"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/scastria/terraform-provider-apigee/apigee/client"
	"mime"
	"net/http"
)

func resourceCache() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCacheCreate,
		ReadContext:   resourceCacheRead,
		UpdateContext: resourceCacheUpdate,
		DeleteContext: resourceCacheDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"environment_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"expiry_timeout_in_sec": {
				Type:          schema.TypeString,
				Optional:      true,
				ConflictsWith: []string{"expiry_time_of_day", "expiry_date"},
			},
			"expiry_time_of_day": {
				Type:          schema.TypeString,
				Optional:      true,
				ConflictsWith: []string{"expiry_timeout_in_sec", "expiry_date"},
			},
			"expiry_date": {
				Type:          schema.TypeString,
				Optional:      true,
				ConflictsWith: []string{"expiry_timeout_in_sec", "expiry_time_of_day"},
			},
			//overflow_to_disk doesn't seem to work in the Apigee API
			//"overflow_to_disk": {
			//	Type:     schema.TypeBool,
			//	Optional: true,
			//	Computed: true,
			//},
			"skip_cache_if_element_size_in_kb_exceeds": {
				Type:     schema.TypeInt,
				Optional: true,
			},
		},
	}
}

func resourceCacheCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	c := m.(*client.Client)
	buf := bytes.Buffer{}
	newCache := client.Cache{
		EnvironmentName: d.Get("environment_name").(string),
		Name:            d.Get("name").(string),
	}
	fillCache(&newCache, d)
	err := json.NewEncoder(&buf).Encode(newCache)
	if err != nil {
		d.SetId("")
		return diag.FromErr(err)
	}
	requestPath := fmt.Sprintf(client.CachePath, c.Organization, newCache.EnvironmentName)
	requestHeaders := http.Header{
		headers.ContentType: []string{mime.TypeByExtension(".json")},
	}
	_, err = c.HttpRequest(http.MethodPost, requestPath, nil, requestHeaders, buf)
	if err != nil {
		d.SetId("")
		return diag.FromErr(err)
	}
	d.SetId(newCache.CacheEncodeId())
	return diags
}

func fillCache(c *client.Cache, d *schema.ResourceData) {
	description, ok := d.GetOk("description")
	if ok {
		c.Description = description.(string)
	}
	expiryTimeoutInSec, ok := d.GetOk("expiry_timeout_in_sec")
	if ok {
		c.ExpirySettings = client.Expiration{
			TimeoutInSec: &client.ExpiryValue{
				Value: expiryTimeoutInSec.(string),
			},
		}
	}
	expiryTimeOfDay, ok := d.GetOk("expiry_time_of_day")
	if ok {
		c.ExpirySettings = client.Expiration{
			TimeOfDay: &client.ExpiryValue{
				Value: expiryTimeOfDay.(string),
			},
		}
	}
	expiryDate, ok := d.GetOk("expiry_date")
	if ok {
		c.ExpirySettings = client.Expiration{
			ExpiryDate: &client.ExpiryValue{
				Value: expiryDate.(string),
			},
		}
	}
	//overflowToDisk, ok := d.GetOk("overflow_to_disk")
	//if ok {
	//	c.OverflowToDisk = overflowToDisk.(bool)
	//}
	skipCacheIfElementSizeInKbExceeds, ok := d.GetOk("skip_cache_if_element_size_in_kb_exceeds")
	if ok {
		c.SkipCacheIfElementSizeInKBExceeds = skipCacheIfElementSizeInKbExceeds.(int)
	}
}

func resourceCacheRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	envName, name := client.CacheDecodeId(d.Id())
	c := m.(*client.Client)
	requestPath := fmt.Sprintf(client.CachePathGet, c.Organization, envName, name)
	body, err := c.HttpRequest(http.MethodGet, requestPath, nil, nil, bytes.Buffer{})
	if err != nil {
		d.SetId("")
		re := err.(*client.RequestError)
		if re.StatusCode == http.StatusNotFound {
			return diags
		}
		return diag.FromErr(err)
	}
	retVal := &client.Cache{}
	err = json.NewDecoder(body).Decode(retVal)
	if err != nil {
		d.SetId("")
		return diag.FromErr(err)
	}
	d.Set("environment_name", envName)
	d.Set("name", name)
	if retVal.Description != "" {
		d.Set("description", retVal.Description)
	}
	timeoutInSec := retVal.ExpirySettings.TimeoutInSec
	timeOfDay := retVal.ExpirySettings.TimeOfDay
	expiryDate := retVal.ExpirySettings.ExpiryDate
	if (timeoutInSec != nil) && (timeoutInSec.Value != "") {
		d.Set("expiry_timeout_in_sec", timeoutInSec.Value)
	} else if (timeOfDay != nil) && (timeOfDay.Value != "") {
		d.Set("expiry_time_of_day", timeOfDay.Value)
	} else if (expiryDate != nil) && (expiryDate.Value != "") {
		d.Set("expiry_date", expiryDate.Value)
	}
	//d.Set("overflow_to_disk", retVal.OverflowToDisk)
	if retVal.SkipCacheIfElementSizeInKBExceeds != 0 {
		d.Set("skip_cache_if_element_size_in_kb_exceeds", retVal.SkipCacheIfElementSizeInKBExceeds)
	}
	return diags
}

func resourceCacheUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	envName, name := client.CacheDecodeId(d.Id())
	c := m.(*client.Client)
	buf := bytes.Buffer{}
	upCache := client.Cache{
		EnvironmentName: envName,
		Name:            name,
	}
	fillCache(&upCache, d)
	err := json.NewEncoder(&buf).Encode(upCache)
	if err != nil {
		return diag.FromErr(err)
	}
	requestPath := fmt.Sprintf(client.CachePathGet, c.Organization, envName, name)
	requestHeaders := http.Header{
		headers.ContentType: []string{mime.TypeByExtension(".json")},
	}
	_, err = c.HttpRequest(http.MethodPut, requestPath, nil, requestHeaders, buf)
	if err != nil {
		return diag.FromErr(err)
	}
	return diags
}

func resourceCacheDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	envName, name := client.CacheDecodeId(d.Id())
	c := m.(*client.Client)
	requestPath := fmt.Sprintf(client.CachePathGet, c.Organization, envName, name)
	_, err := c.HttpRequest(http.MethodDelete, requestPath, nil, nil, bytes.Buffer{})
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId("")
	return diags
}