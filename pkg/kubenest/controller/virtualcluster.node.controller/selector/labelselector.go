package selector

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

type ParserMatchFunc func(matchExpression metav1.LabelSelectorRequirement, metaLabels labels.Set) bool

func GetFunc(operator metav1.LabelSelectorOperator) (ParserMatchFunc, error) {
	switch operator {
	case metav1.LabelSelectorOpDoesNotExist:
		return ParseWithLabelSelectorOpDoesNotExist, nil
	case metav1.LabelSelectorOpExists:
		return ParseWithLabelSelectorOpExists, nil
	case metav1.LabelSelectorOpNotIn:
		return ParseWithLabelSelectorOpNotIn, nil
	case metav1.LabelSelectorOpIn:
		return ParseWithLabelSelectorOpIn, nil
	default:
		return nil, fmt.Errorf("unsupported operator %s", operator)
	}
}

func MatchesWithLabelSelector(metaLabels labels.Set, labelSelector *metav1.LabelSelector) (bool, error) {
	if !util.MapContains(metaLabels, labelSelector.MatchLabels) {
		return false, nil
	}

	for _, expr := range labelSelector.MatchExpressions {
		parseMatchFunc, err := GetFunc(expr.Operator)
		if err != nil {
			return false, err
		}
		if !parseMatchFunc(expr, metaLabels) {
			return false, nil
		}
	}
	return true, nil
}

func ParseWithLabelSelectorOpDoesNotExist(matchExpression metav1.LabelSelectorRequirement, metaLabels labels.Set) bool {
	return !metaLabels.Has(matchExpression.Key)
}

func ParseWithLabelSelectorOpExists(matchExpression metav1.LabelSelectorRequirement, metaLabels labels.Set) bool {
	return metaLabels.Has(matchExpression.Key)
}

func ParseWithLabelSelectorOpNotIn(matchExpression metav1.LabelSelectorRequirement, metaLabels labels.Set) bool {
	if !metaLabels.Has(matchExpression.Key) || !contains(matchExpression.Values, metaLabels[matchExpression.Key]) {
		return true
	}
	return false
}

func ParseWithLabelSelectorOpIn(matchExpression metav1.LabelSelectorRequirement, metaLabels labels.Set) bool {
	if metaLabels.Has(matchExpression.Key) && contains(matchExpression.Values, metaLabels[matchExpression.Key]) {
		return true
	}
	return false
}

func contains(arr []string, s string) bool {
	for _, str := range arr {
		if str == s {
			return true
		}
	}
	return false
}
