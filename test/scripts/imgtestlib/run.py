import os
import subprocess as sp
import sys


def runcmd(cmd, stdin=None, extra_env=None, capture_output=True):
    """
    Run the cmd using sp.run() and exit with the returncode if it is non-zero.
    Output is captured and both stdout and stderr are returned if the run succeeds.
    If it fails, the output is printed before exiting.
    """
    env = None
    if extra_env:
        env = os.environ
        env.update(extra_env)
    job = sp.run(cmd, input=stdin, capture_output=capture_output, env=env, check=False)
    if job.returncode > 0:
        print(f"❌ Command failed: {cmd}")
        if job.stdout:
            print(job.stdout.decode())
        if job.stderr:
            print(job.stderr.decode())
        sys.exit(job.returncode)

    return job.stdout, job.stderr


def runcmd_nc(cmd, stdin=None, extra_env=None):
    """
    Run the cmd using sp.run() and exit with the returncode if it is non-zero.
    Output it not captured.
    """
    runcmd(cmd, stdin=stdin, extra_env=extra_env, capture_output=False)
