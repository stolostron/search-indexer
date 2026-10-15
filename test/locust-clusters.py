from locust import HttpUser, task, between, TaskSet
import copy
import time
import json
import uuid
import urllib3
import io
urllib3.disable_warnings() # Suppress warning from unverified TLS connection (verify=false)

clusterCount = 0 # Used to name clusters sequentially.

class ClusterBehavior(TaskSet):
# A cluster sends 2 types of messages:
#  1. Full state sync. This typically happens with the first request.
#  2. Update state.

    def send_full_state_payload(self):
        with open("cluster-templates/{}".format(self.user.template), "r") as template_file:
            template_string = template_file.read().replace("<<CLUSTER_NAME>>", self.user.name)
        f = io.StringIO(template_string)
        j = json.load(f)        
        self.client.payload = j
        self.client.headers["X-Overwrite-State"] = "true"
        self.requestName = self.user.template
        self.do_post()


    # Build a payload with the desired number of add and update resources.
    def send_update_payload(self):
        print("%10s - Sending update" % self.user.name)
        f = open("cluster-templates/add-update-template.json", "r")
        j = json.load(f)
        adds = []
        updates = []
        # Replace add resource template with desired number of resources.
        add_template = j["addResources"][0]
        for i in range(self.user.updatesPerCycle):
            r = copy.deepcopy(add_template)
            r["uid"] = r["uid"].replace("<<CLUSTER_NAME>>", self.user.name)
            r["uid"] = r["uid"].replace("<<UID>>", str(i) )
            r["properties"]["name"] = r["properties"]["name"].replace("<<UID>>", str(i) )
            r["properties"]["label"]["update-label"] = "Added at {}".format(str(time.time()) )
            adds.append(r)
        j["addResources"] = adds

        # Replace update resource template with desired number of resources.
        update_template = j["updateResources"][0]
        for i in range(self.user.updatesPerCycle):
            r = copy.deepcopy(update_template)
            r["uid"] = r["uid"].replace("<<CLUSTER_NAME>>", self.user.name)
            r["uid"] = r["uid"].replace("<<UID>>", str(i) )
            r["properties"]["name"] = r["properties"]["name"].replace("<<UID>>", str(i) )
            r["properties"]["label"]["update-label"] = "Updated at {}".format(str(time.time()) )
            updates.append(r)
        j["updateResources"] = updates
        self.client.payload = j
        self.client.headers["X-Overwrite-State"] = "false"
        self.requestName = "delta-add-update"
        self.do_post()

    # Send a payload to the cluster.
    def do_post(self):
        resp = self.client.post("/aggregator/clusters/{}/sync".format(self.user.name), name=self.requestName, json=self.client.payload, verify=False)
        if resp.status_code != 200: # The first request is receiving 0 instead of 429.
            self.user.retries = self.user.retries + 1
            print("%10s -\t Received response code %s. Waiting %d seconds and retrying." % (self.user.name, resp.status_code, self.user.retries * 2))
            time.sleep(self.user.retries * 2)
            self.do_post()
        else:
            print("%10s -\t Completed do_post() with %s retries." % (self.user.name, self.user.retries))
            self.user.retries = 0

    def on_start(self):
        self.send_full_state_payload()
        # self.send_update_payload()

    @task(50)
    def send_update(self):
        self.send_update_payload()

    @task(1)
    def send_resync(self):
        self.send_full_state_payload()


class Cluster(HttpUser):
    abstract = True
    tasks = [ClusterBehavior]
    wait_time = between(5, 10)
    retries = 0
    def on_start(self):
        global clusterCount
        self.name = "locust-{}".format(clusterCount)
        clusterCount = clusterCount + 1
        print("%10s - Starting cluster simulation using template %s" % (self.name, self.template))
        

# Cluster5k - simulates 5k resources.
class Cluster5k(Cluster):
    template = "sno-5k.json"
    updatesPerCycle = 10

# Cluster100k - simulates 100k resources.
class Cluster100k(Cluster):
    template = "sno-100k.json"
    updatesPerCycle = 20

# Cluster150k - simulates 150k resources.
class Cluster150k(Cluster):
    template = "sno-150k.json"
    updatesPerCycle = 30
