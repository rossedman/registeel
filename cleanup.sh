#!/bin/bash

NAMESPACES=$(kubectl get ns -o jsonpath='{.items[*].metadata.name}')
for NAMESPACE in $NAMESPACES; do
    DEPLOYMENTS=$(kubectl get deployments -n $NAMESPACE -o jsonpath='{.items[*].metadata.name}')
    for DEPLOY in $DEPLOYMENTS; do
        kubectl annotate deploy/$DEPLOY -n $NAMESPACE app.registeel.io/registered-
        kubectl annotate deploy/$DEPLOY -n $NAMESPACE app.registeel.io/last-updated-
        kubectl annotate deploy/$DEPLOY -n $NAMESPACE app.registeel.io/last-version-
    done
done
