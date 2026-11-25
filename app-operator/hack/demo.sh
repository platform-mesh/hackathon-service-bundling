#!/usr/bin/env bash
# Demo of the app operator on a local kcp:
# starts kcp, exports the App API from the root workspace, starts the
# controller with kcp's admin kubeconfig, then creates, deletes and re-creates
# the sample App in a "demo" workspace. Keeps kcp and the controller running
# until interrupted.
set -euo pipefail

cd "$(dirname "$0")/.."

KCP_DIR=.kcp # default root directory of `kcp start`
KCP_LOG="$KCP_DIR/kcp.log"
EXPORT=juiceshop.com
SAMPLE=config/samples/juiceshop_v1alpha1_app.yaml
PAUSE="${DEMO_PAUSE:-3}" # seconds between steps, so the controller logs are readable

export KUBECONFIG="$PWD/$KCP_DIR/admin.kubeconfig"

kcp=""
controller=""

cleanup() {
	for pid in "$controller" "$kcp"; do
		if [ -n "$pid" ]; then kill "$pid" 2>/dev/null || true; fi
	done
	wait 2>/dev/null || true
}
trap cleanup EXIT
trap 'exit 0' INT TERM # stopping the demo is the expected way out

step() { printf '\n==> %s\n' "$*"; }

# retry CMD...: re-run CMD until it succeeds, for up to two minutes.
retry() {
	local deadline=$((SECONDS + 120))
	until "$@" >/dev/null 2>&1; do
		if ! kill -0 "$kcp" 2>/dev/null; then
			echo "kcp exited, last lines of $KCP_LOG:" >&2
			tail -n 20 "$KCP_LOG" >&2
			exit 1
		fi
		if [ "$SECONDS" -ge "$deadline" ]; then
			echo "timed out waiting for: $*" >&2
			exit 1
		fi
		sleep 1
	done
}

step "Starting kcp (logs in $KCP_LOG)"
mkdir -p "$KCP_DIR"
kcp start >"$KCP_LOG" 2>&1 &
kcp=$!

step "Waiting for kcp to be ready"
retry kubectl get --raw /readyz
retry kubectl get workspacetype universal

step "Exporting the App API from the root workspace"
retry kubectl apply -f config/kcp
kubectl get apiexport "$EXPORT"
retry kubectl get apiexportendpointslice "$EXPORT"

step "Starting the controller with $KCP_DIR/admin.kubeconfig"
./bin/manager --endpointslice="$EXPORT" &
controller=$!

sleep "$PAUSE"
if ! kill -0 "$controller" 2>/dev/null; then
	echo "controller exited during startup" >&2
	exit 1
fi

step "Creating the demo workspace and binding the App API in it"
kubectl apply -f config/demo/workspace.yaml
kubectl wait --for=jsonpath='{.status.phase}'=Ready --timeout=60s workspace/demo
demo_url="$(kubectl get workspace demo -o jsonpath='{.spec.URL}')"
kdemo() { kubectl --server "$demo_url" "$@"; }
kdemo apply -f config/demo/apibinding.yaml
kdemo wait --for=condition=Ready --timeout=60s apibinding/"$EXPORT"
sleep "$PAUSE"

step "Creating the sample App"
kdemo apply -f "$SAMPLE"
sleep "$PAUSE"

step "Deleting the sample App"
kdemo delete -f "$SAMPLE"
sleep "$PAUSE"

step "Re-creating the sample App"
kdemo apply -f "$SAMPLE"
sleep "$PAUSE"

step "Demo done, kcp and the controller keep running (Ctrl-C to stop)"
echo "    kubeconfig:     $KUBECONFIG"
echo "    demo workspace: $demo_url"

while kill -0 "$kcp" 2>/dev/null && kill -0 "$controller" 2>/dev/null; do
	sleep 1
done
echo "kcp or the controller exited unexpectedly" >&2
exit 1
