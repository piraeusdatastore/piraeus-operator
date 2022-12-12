package conditions

import (
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	CondType string
	Reason   string
)

const (
	Applied    CondType = "Applied"
	Available  CondType = "Available"
	Configured CondType = "Configured"

	ReasonNotObserved Reason = "NotObserved"
	ReasonAsExpected  Reason = "AsExpected"
	ReasonUnknown     Reason = "Unknown"
	ReasonError       Reason = "Error"
)

type condition struct {
	Status  metav1.ConditionStatus
	Reason  Reason
	Errors  []error
	Message []string
}

type Conditions map[CondType]*condition

func New() Conditions {
	return map[CondType]*condition{
		Applied:    {Reason: ReasonNotObserved, Status: metav1.ConditionUnknown},
		Available:  {Reason: ReasonNotObserved, Status: metav1.ConditionUnknown},
		Configured: {Reason: ReasonNotObserved, Status: metav1.ConditionUnknown},
	}
}

func (c Conditions) get(condType CondType) *condition {
	res, ok := c[condType]
	if ok {
		return res
	}

	c[condType] = &condition{Reason: ReasonNotObserved, Status: metav1.ConditionUnknown}
	return c[condType]
}

func (c Conditions) AddSuccess(condType CondType, message string) {
	ct := c.get(condType)
	ct.Status = metav1.ConditionTrue
	ct.Reason = ReasonAsExpected
	ct.Message = append(ct.Message, message)
}

func (c Conditions) AddError(condType CondType, err error) {
	ct := c.get(condType)
	ct.Status = metav1.ConditionFalse
	ct.Reason = ReasonError
	ct.Message = append(ct.Message, fmt.Sprintf("Error: %v", err))
}

func (c Conditions) AddUnknown(condType CondType, message string) {
	ct := c.get(condType)
	ct.Status = metav1.ConditionUnknown
	ct.Reason = ReasonUnknown
	ct.Message = append(ct.Message, message)
}

func (c Conditions) ToConditions(generation int64) []metav1.Condition {
	var result []metav1.Condition
	for k, v := range c {
		message := strings.Join(v.Message, "\n")

		result = append(result, metav1.Condition{
			Type:               string(k),
			Status:             v.Status,
			Reason:             string(v.Reason),
			Message:            message,
			ObservedGeneration: generation,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Type < result[j].Type
	})

	return result
}
