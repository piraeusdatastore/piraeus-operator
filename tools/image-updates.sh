#!/bin/sh -e

YQ="${YQ:-yq}"
KUSTOMIZE="${KUSTOMIZE:-kustomize}"
CRANE="${CRANE:-crane}"

NEED_CHART_UPDATE=n

for CONFIG_FILE in ./config/manager/*_images.yaml; do
	BASE="$(yq -e .base "${CONFIG_FILE}")"
	for KEY in $(yq -e ".components | keys | .[]" "${CONFIG_FILE}"); do
		IMG="${BASE}/$(yq eval -e ".components.${KEY}.image" "${CONFIG_FILE}")"
		TAG="$(crane ls "${IMG}" | grep -P '^v(\d+\.)?(\d+\.)?(\*|\d+)$' | sort --version-sort | tail -1)"
		if [ "$(yq -e ".components.${KEY}.tag" "${CONFIG_FILE}" )" != "${TAG}" ] ; then
			echo "⬆ ${IMG}"
			yq -ie ".components.${KEY}.tag = \"${TAG}\"" "${CONFIG_FILE}"
			NEED_CHART_UPDATE=y
		else
			echo "= ${IMG}"
		fi
	done
done

if [ "$NEED_CHART_UPDATE" = y ]; then
	make sync-chart
fi
