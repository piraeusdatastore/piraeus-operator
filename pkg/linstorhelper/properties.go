package linstorhelper

import (
	"encoding/json"
	"sort"

	linstor "github.com/LINBIT/golinstor"
	lclient "github.com/LINBIT/golinstor/client"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

const (
	LastApplyProperty = linstor.NamespcAuxiliary + "/" + vars.ApplyAnnotation
	ManagedByProperty = linstor.NamespcAuxiliary + "/" + vars.ManagedByLabel
)

// MakePropertiesModification returns the modification that need to be applied to update the current properties.
func MakePropertiesModification(current, expected map[string]string) *lclient.GenericPropsModify {
	modify := false
	result := &lclient.GenericPropsModify{
		OverrideProps: make(map[string]string),
	}

	expected = UpdateLastApplyProperty(expected)

	for k, v := range expected {
		if current, ok := current[k]; !ok || current != v {
			modify = true
			result.OverrideProps[k] = v
		}
	}

	// Check what properties we applied last to find if we need to delete something
	if !modify && current[LastApplyProperty] == expected[LastApplyProperty] {
		return nil
	}

	// See what properties need to be deleted from the previous run.
	var lastApplied []string
	_ = json.Unmarshal([]byte(current[LastApplyProperty]), &lastApplied)

	for _, k := range lastApplied {
		if _, ok := expected[k]; !ok {
			result.DeleteProps = append(result.DeleteProps, k)
		}
	}

	return result
}

// UpdateLastApplyProperty ensures the LastApplyProperty is up-to-date.
func UpdateLastApplyProperty(props map[string]string) map[string]string {
	result := make(map[string]string)
	allKeys := make([]string, 0)

	for k, v := range props {
		if k != LastApplyProperty {
			result[k] = v
			allKeys = append(allKeys, k)
		}
	}

	// Sort for consistent order
	sort.Strings(allKeys)

	toApply, err := json.Marshal(allKeys)
	if err != nil {
		// pretty much can't happen, and if it happens, you can't apply the properties.
		return result
	}

	result[LastApplyProperty] = string(toApply)
	return result
}
