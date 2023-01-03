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

var conditionPriority = map[Reason]int{
	ReasonNotObserved: 0,
	ReasonAsExpected:  1,
	ReasonUnknown:     2,
	ReasonError:       3,
}

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
	ct.Message = append(ct.Message, message)
	if conditionPriority[ct.Reason] < conditionPriority[ReasonAsExpected] {
		ct.Status = metav1.ConditionTrue
		ct.Reason = ReasonAsExpected
	}
}

func (c Conditions) AddError(condType CondType, err error) {
	ct := c.get(condType)
	ct.Message = append(ct.Message, fmt.Sprintf("Error: %v", err))
	if conditionPriority[ct.Reason] < conditionPriority[ReasonError] {
		ct.Status = metav1.ConditionFalse
		ct.Reason = ReasonError
	}
}

func (c Conditions) AddUnknown(condType CondType, message string) {
	ct := c.get(condType)
	ct.Message = append(ct.Message, message)
	if conditionPriority[ct.Reason] < conditionPriority[ReasonUnknown] {
		ct.Status = metav1.ConditionUnknown
		ct.Reason = ReasonUnknown
	}
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

func (c Conditions) AllHaveStatus(status metav1.ConditionStatus) bool {
	for _, cond := range c {
		if cond.Status != status {
			return false
		}
	}

	return true
}
