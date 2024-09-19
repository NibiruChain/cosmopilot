#!/usr/bin/env bash

CLUSTER_NAME=cosmopilot
ISSUER_NAME=letsencrypt-prod
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
KIND_BIN=$(which kind)
KUBECTL_BIN=$(which kubectl)
HELM_BIN=$(which helm)

usage()
{
    echo -e "Manage cosmopilot local environment\n"
    echo -e "Usage: \n  $0 [command]\n"
    echo -e "Available Commands:"
    echo -e "  up \t Setup the environment"
    echo -e "  down \t Tear down the environment"
    echo -e "  curl \t Use curl to access cluster exposed ingresses"
    echo -e "\nFlags:"
    echo -e "  -n, --cluster-name \t Kind cluster name"
    echo -e "  -i, --issuer-name \t Cert-manager cluster issuer name"
    echo -e "  -c, --kind-bin \t Path to kind binary"
    echo -e "  -k, --kubectl-bin \t Path to kubectl binary"
    echo -e "  -e, --helm-bin \t Path to helm binary"
    echo -e "  -h, --help \t\t Show this menu\n"
    exit 1
}

red=`tput setaf 1`
green=`tput setaf 2`
reset=`tput sgr0`

assert_executable_exists()
{
    if ! command -v $1 &> /dev/null
    then
        echo -e "${red}Error:${reset} $1 could not be found. Please install it and re-run this script."
        exit
    fi
}

POSITIONAL=()
while [[ $# -gt 0 ]]
do
key="$1"

case $key in
    -n|--cluster-name)
    CLUSTER_NAME="$2"
    shift
    shift
    ;;
    -i|--issuer-name)
    ISSUER_NAME="$2"
    shift
    shift
    ;;
    -c|--kind-bin)
    KIND_BIN="$2"
    shift
    shift
    ;;
    -k|--kubectl-bin)
    KUBECTL_BIN="$2"
    shift
    shift
    ;;
    -e|--helm-bin)
    HELM_BIN="$2"
    shift
    shift
    ;;
    -h|--help)
    usage
    shift
    ;;
    *)
    POSITIONAL+=("$1")
    shift
    ;;
esac
done
set -- "${POSITIONAL[@]}"

COMMAND=${POSITIONAL[0]}

if [ "$COMMAND" = "" ]
then
    usage
fi

if [[ ! "$COMMAND" =~ ^(up|down|curl)$ ]]
then
    echo -e "${red}Error:${reset} command does not exist\n"
    usage
fi


if [ "$COMMAND" = "up" ]
then
    if $KIND_BIN get clusters | grep $CLUSTER_NAME &> /dev/null
    then
        echo -e "${green}\xE2\x9C\x94${reset} Cluster $CLUSTER_NAME already exists"
    else
        echo -e "${green}\xE2\x9C\x94${reset} Creating cluster $CLUSTER_NAME"
        $KIND_BIN create cluster --name $CLUSTER_NAME --config=${SCRIPT_DIR}/kind.yml
    fi

    # Install nginx
    echo -e "${green}\xE2\x9C\x94${reset} Ensure nginx ingress controller is installed and running"
    $KUBECTL_BIN apply \
      --context kind-$CLUSTER_NAME \
      -f  https://raw.githubusercontent.com/kubernetes/ingress-nginx/helm-chart-4.11.2/deploy/static/provider/kind/deploy.yaml \
      &> /dev/null
    $KUBECTL_BIN patch \
      --context kind-$CLUSTER_NAME \
      --namespace ingress-nginx \
      svc ingress-nginx-controller \
      --patch "$(cat ${SCRIPT_DIR}/nginx-patch.yml)" \
      &> /dev/null

    # Install cert-manager
    echo -e "${green}\xE2\x9C\x94${reset} Ensure cert-manager is installed and running"
    $KUBECTL_BIN apply \
      --context kind-$CLUSTER_NAME \
      --force-conflicts=true \
      --server-side \
      -f https://github.com/cert-manager/cert-manager/releases/download/v1.15.3/cert-manager.yaml  \
      &> /dev/null

    ### Wait for cert-manager to be up and running
    while : ; do
        $KUBECTL_BIN get pod \
            --context kind-$CLUSTER_NAME \
            --namespace ingress-nginx \
            --selector=app.kubernetes.io/component=controller 2>&1 | grep -q controller && break
        sleep 2
    done
    $KUBECTL_BIN wait pod \
        --context kind-$CLUSTER_NAME \
        --namespace cert-manager \
        --for=condition=ready \
        --selector=app.kubernetes.io/component=webhook \
        --timeout=90s \
        &> /dev/null

    echo -e "${green}\xE2\x9C\x94${reset} Ensure $ISSUER_NAME ClusterIssuer exists"
    $KUBECTL_BIN apply \
      --context kind-$CLUSTER_NAME \
      -f - &> /dev/null <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: $ISSUER_NAME
spec:
  selfSigned: {}
EOF
fi

if [ "$COMMAND" = "down" ]
then
    if $KIND_BIN get clusters | grep $CLUSTER_NAME &> /dev/null
    then
        echo -e "${green}\xE2\x9C\x94${reset} Deleting cluster $CLUSTER_NAME"
        $KIND_BIN delete cluster --name $CLUSTER_NAME &> /dev/null
    fi
fi

if [ "$COMMAND" = "curl" ]
then
    curl --resolve *:443:127.0.0.1 -k ${POSITIONAL[@]:1}
fi
