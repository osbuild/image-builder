import os
import json
from locust import HttpUser, task, tag, events

COMPOSE_FAIL_RATIO = float(os.getenv("COMPOSE_LT_FAIL_RATIO", "0.01"))
COMPOSE_PERCENTILE_95_RESPONSE_TIME = int(os.getenv("COMPOSE_LT_PERCENTILE_95_RESPONSE_TIME",
    "6000"))

FAIL_RATIO = float(os.getenv("LT_FAIL_RATIO", "0.01"))
MEAN_RESPONSE_TIME = int(os.getenv("LT_MEAN_RESPONSE_TIME", "200"))
MEDIAN_RESPONSE_TIME = int(os.getenv("LT_MEDIAN_RESPONSE_TIME", "280"))
PERCENTILE_95_RESPONSE_TIME = int(os.getenv("LT_PERCENTILE_95_RESPONSE_TIME",
    "500"))

INPUT_JSON={
    "distribution": "centos-9",
    "image_requests": [
        {
            "architecture": "x86_64",
            "image_type": "ami",
            "upload_request": {
                "type": "aws",
                "options": {
                    "share_with_accounts": ["somebody"]
                }
            }
        }
    ],
    "customizations":{
        "packages": [
            "idontexist"
        ]
    }
}

class LoadTestImageBuilderComposer(HttpUser):
    """
    Perform the load testing of image builder
    """

    def on_start(self):
        self.client.proxies = {
                "http":"http://squid.corp.redhat.com:3128/",
                "https":"http://squid.corp.redhat.com:3128/"
                }

    @task
    def test_compose(self):
        self.client.post("/compose", json=INPUT_JSON)

class LoadTestImageBuilder(HttpUser):
    """
    Perform the load testing of image builder
    """

    def on_start(self):
        self.client.proxies = {
                "http":"http://squid.corp.redhat.com:3128/",
                "https":"http://squid.corp.redhat.com:3128/"
                }

    @task
    def test_packages(self):
        """
        Test the packages endpoint by making it search for qemu
        """
        self.client.get("/packages?distribution=rhel-84"
                "&architecture=x86_64&search=qemu")

    @task
    def test_distributions(self):
        """
        Test the distributions endpoint and for each distribution test the
        architectures endpoint
        """
        response = self.client.get("/distributions")
        distributions = json.loads(response.content.decode("utf-8"))
        for distribution in distributions:
            self.client.get(f"/architectures/{distribution['name']}")

    @tag("debug")
    @task
    def test_version(self):
        """
        Test the version endpoint, can be used to know if a slow response is due
        to network issues (for debug purposes)
        """
        self.client.get("/version")

@events.quitting.add_listener
def _(environment, **kw):
    """
    Upon quitting, test the stats of the load test. The response time and error
    rate muse be below some threshold, otherwise the test is considered failed.
    """
    environment.process_exit_code = 0
    for key, value in environment.stats.entries.items():
        # Test separately the compose request and the other ones as they are not
        # expected to have the same latencies.
        if key[0] == '/api/image-builder/v1/compose':
            if value.fail_ratio > COMPOSE_FAIL_RATIO:
                print(f"{key} Test failed due to failure ratio "
                        f"> {COMPOSE_FAIL_RATIO}%")
                environment.process_exit_code = 1
            elif (value.get_response_time_percentile(0.95) >
                    COMPOSE_PERCENTILE_95_RESPONSE_TIME):
                print(f"{key} Test failed due to 95th percentile response time > "
                        f"{COMPOSE_PERCENTILE_95_RESPONSE_TIME} ms")
                environment.process_exit_code = 1
        else:
            if value.fail_ratio > FAIL_RATIO:
                print(f"{key} Test failed due to failure ratio > {FAIL_RATIO}%")
                environment.process_exit_code = 1
            elif value.avg_response_time > MEAN_RESPONSE_TIME:
                print(f"{key} Test failed due to average response time ratio > "
                        f"{MEAN_RESPONSE_TIME} ms")
                environment.process_exit_code = 1
            elif (value.get_response_time_percentile(0.5) >
                    MEDIAN_RESPONSE_TIME):
                print(f"{key} Test failed due to average response time ratio > "
                        f"{MEDIAN_RESPONSE_TIME} ms")
                environment.process_exit_code = 1
            elif (value.get_response_time_percentile(0.95) >
                    PERCENTILE_95_RESPONSE_TIME):
                print(f"{key} Test failed due to 95th percentile response time "
                        f"> {PERCENTILE_95_RESPONSE_TIME} ms")
                environment.process_exit_code = 1
    if environment.process_exit_code == 0:
        print("Composer is fast enough 🚀")
