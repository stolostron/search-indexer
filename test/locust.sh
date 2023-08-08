#!/bin/bash
cd test

locust --headless --users ${N_CLUSTERS} --spawn-rate ${SPAWN_RATE} -H https://${HOST} -P 8085 -f locust-clusters.py 