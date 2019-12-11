#!/usr/bin/env python3

import os
import re
import sys
import json
import time
import select
import shutil
import logging
import argparse
import threading
import subprocess
import urllib.request

ETCD_IMAGE = "quay.io/coreos/etcd:v3.3.5"

LOG_LEVELS = {
    "critical": logging.CRITICAL,
    "error": logging.ERROR,
    "warning": logging.WARNING,
    "info": logging.INFO,
    "debug": logging.DEBUG,
}

LOG_COLORS = {
    "critical": "\x1b[31;1m",
    "error": "\x1b[31;1m",
    "warning": "\x1b[33;1m",
}

LOCAL_CONFIG_PATTERN = re.compile(r"^local(-\d+)?$")

log = logging.getLogger(__name__)
handler = logging.StreamHandler()
handler.setFormatter(logging.Formatter("%(levelname)s:%(message)s"))
log.addHandler(handler)

class ExcThread(threading.Thread):
    def __init__(self, target):
        super().__init__(target=target)
        self.error = None

    def run(self):
        try:
            self._target()
        except Exception as e:
            self.error = e

def join(*targets):
    threads = []

    for target in targets:
        t = ExcThread(target)
        t.start()
        threads.append(t)

    for t in threads:
        t.join()
    for t in threads:
        if t.error is not None:
            raise Exception("Thread error") from t.error

class Output:
    def __init__(self, pipe, level):
        self.pipe = pipe
        self.level = level
        self.lines = []

class ProcessResult:
    def __init__(self, rc, stdout, stderr):
        self.rc = rc
        self.stdout = stdout
        self.stderr = stderr

class DefaultDriver:
    def available(self):
        return True

    def clear(self):
        run("kubectl", "delete", "daemonsets,replicasets,services,deployments,pods,rc,pvc", "--all")

    def start(self):
        pass

    def push_images(self, deploy_version, dash_image):
        pass

    def wait(self):
        while suppress("pachctl", "version") != 0:
            log.info("Waiting for pachyderm to come up...")
            time.sleep(1)

    def set_config(self):
        pass

class DockerDriver(DefaultDriver):
    def set_config(self):
        run("pachctl", "config", "update", "context", "--pachd-address=localhost:30650")

class MinikubeDriver(DefaultDriver):
    def available(self):
        return run("which", "minikube", raise_on_error=False).rc == 0

    def clear(self):
        run("minikube", "delete")

    def start(self):
        run("minikube", "start")

        while suppress("minikube", "status") != 0:
            log.info("Waiting for minikube to come up...")
            time.sleep(1)

    def push_images(self, deploy_version, dash_image):
        run("./etc/kube/push-to-minikube.sh", "pachyderm/pachd:{}".format(deploy_version))
        run("./etc/kube/push-to-minikube.sh", "pachyderm/worker:{}".format(deploy_version))
        run("./etc/kube/push-to-minikube.sh", ETCD_IMAGE)
        run("./etc/kube/push-to-minikube.sh", dash_image)

    def set_config(self):
        ip = capture("minikube", "ip")
        run("pachctl", "config", "update", "context", "--pachd-address={}".format(ip))

def parse_log_level(s):
    try:
        return LOG_LEVELS[s]
    except KeyError:
        raise Exception("Unknown log level: {}".format(s))

def find_in_json(j, f):
    if f(j):
        return j
    elif isinstance(j, dict):
        for v in j.values():
            find_in_json(v, f)
    elif isinstance(j, list):
        for i in j:
            find_in_json(i, f)

def run(cmd, *args, raise_on_error=True, stdout_log_level="info", stderr_log_level="error", stdin=None):
    log.debug("Running `%s %s`", cmd, " ".join(args))

    proc = subprocess.Popen([cmd, *args], shell=False, stdout=subprocess.PIPE, stderr=subprocess.PIPE, stdin=subprocess.PIPE if stdin is not None else None)
    stdout = Output(proc.stdout, stdout_log_level)
    stderr = Output(proc.stderr, stderr_log_level)
    timed_out_last = False

    if stdin is not None:
        proc.stdin.write(stdin)

    while True:
        if (proc.poll() is not None and timed_out_last) or (stdout.pipe.closed and stderr.pipe.closed):
            break

        for io in select.select([stdout.pipe, stderr.pipe], [], [], 100)[0]:
            timed_out_last = False
            line = io.readline().decode().rstrip()

            if line == "":
                continue

            dest = stdout if io == stdout.pipe else stderr
            log.log(LOG_LEVELS[dest.level], "{}{}\x1b[0m".format(LOG_COLORS.get(dest.level, ""), line))
            dest.lines.append(line)
        else:
            timed_out_last = True

    rc = proc.wait()

    if raise_on_error and rc != 0:
        raise Exception("Unexpected return code for `{} {}`: {}".format(cmd, " ".join(args), rc))

    return ProcessResult(rc, "\n".join(stdout.lines), "\n".join(stderr.lines))

def capture(cmd, *args):
    return run(cmd, *args, stdout_log_level="debug").stdout

def suppress(cmd, *args):
    return run(cmd, *args, stdout_log_level="debug", stderr_log_level="debug", raise_on_error=False).rc

def rewrite_config():
    log.info("Rewriting config")

    keys = set([])

    try:
        with open(os.path.expanduser("~/.pachyderm/config.json"), "r") as f:
            j = json.load(f)
    except:
        return
        
    v2 = j.get("v2")
    if not v2:
        return

    contexts = v2["contexts"]

    for k, v in contexts.items():
        if LOCAL_CONFIG_PATTERN.fullmatch(k) and len(v) > 0:
            keys.add(k)

    for k in keys:
        del contexts[k]

    with open(os.path.expanduser("~/.pachyderm/config.json"), "w") as f:
        json.dump(j, f, indent=2)

def main():
    parser = argparse.ArgumentParser(description="Recompiles pachyderm tooling and restarts the cluster with a clean slate.")
    parser.add_argument("--no-deploy", default=False, action="store_true", help="Disables deployment")
    parser.add_argument("--no-config-rewrite", default=False, action="store_true", help="Disables config rewriting")
    parser.add_argument("--deploy-args", default="", help="Arguments to be passed into `pachctl deploy`")
    parser.add_argument("--deploy-to", default="local", help="Set where to deploy")
    parser.add_argument("--deploy-version", default="head", help="Sets the deployment version")
    parser.add_argument("--log-level", default="info", type=parse_log_level, help="Sets the log level; defaults to 'info', other options include 'critical', 'error', 'warning', and 'debug'")
    args = parser.parse_args()

    log.setLevel(args.log_level)

    if "GOPATH" not in os.environ:
        log.critical("Must set GOPATH")
        sys.exit(1)
    if not args.no_deploy and "PACH_CA_CERTS" in os.environ:
        log.critical("Must unset PACH_CA_CERTS\nRun:\nunset PACH_CA_CERTS", file=sys.stderr)
        sys.exit(1)

    if args.deploy_to == "local":
        if MinikubeDriver().available():
            log.info("using the minikube driver")
            driver = MinikubeDriver()
        else:
            log.info("using the k8s for docker driver")
            log.warning("with this driver, it's not possible to fully reset the cluster")
            driver = DockerDriver()
    else:
        driver = DefaultDriver()

    driver.clear()

    gopath = os.environ["GOPATH"]

    if args.deploy_version == "head":
        try:
            os.remove(os.path.join(gopath, "bin", "pachctl"))
        except:
            pass

        procs = [
            driver.start,
            lambda: run("make", "install"),
            lambda: run("make", "docker-build"),
        ]

        if not args.no_config_rewrite:
            procs.append(rewrite_config)

        join(*procs)
    else:
        should_download = suppress("which", "pachctl") != 0 \
            or capture("pachctl", "version", "--client-only") != args.deploy_version

        if should_download:
            release_url = "https://github.com/pachyderm/pachyderm/releases/download/v{}/pachctl_{}_{}_amd64.tar.gz".format(args.deploy_version, args.deploy_version, sys.platform)
            bin_path = os.path.join(os.environ["GOPATH"], "bin")

            with urllib.request.urlopen(release_url) as response:
                with gzip.GzipFile(fileobj=response) as uncompressed:
                    with open(bin_path, "wb") as f:
                        shutil.copyfileobj(uncompressed, f)

        run("docker", "pull", "pachyderm/pachd:{}".format(args.deploy_version))
        run("docker", "pull", "pachyderm/worker:{}".format(args.deploy_version))

    version = capture("pachctl", "version", "--client-only")
    log.info("Deploy pachyderm version v{}".format(version))

    run("which", "pachctl")

    deployments = json.loads("[{}]".format(capture("pachctl", "deploy", "local", "-d", "--dry-run").replace("}\n{", "},{")))

    dash_image = find_in_json(deployments, lambda j: isinstance(j, dict) and j.get("name") == "dash" and j.get("image") is not None)["image"]
    grpc_proxy_image = find_in_json(deployments, lambda j: isinstance(j, dict) and j.get("name") == "grpc-proxy")["image"]

    run("docker", "pull", dash_image)
    run("docker", "pull", grpc_proxy_image)
    run("docker", "pull", ETCD_IMAGE)
    driver.push_images(args.deploy_version, dash_image)

    if not args.no_deploy:
        if args.deploy_to != "local":
            deployments = deployments.replace("local", args.deploy_to)

        run("kubectl", "create", "-f", "-", stdin=deployments)
        driver.wait()

    driver.set_config()

if __name__ == "__main__":
    main()
