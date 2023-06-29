# search-indexer
Index resources for search.


## Configuration and environment variables
Available environment variables are documented within the code at ./pkg/config/config.go

## Logging guidelines
Change log verbosity by passing the argument `-v=9`. Verbosity values are 0-9.

| Log Level | Output                                                  | Example
| Fatal     | Critical unrecoverable error, will terminate execution. | missing required env variables
| Error     | PPotential problem or unrecoverable error.              | DB auth problem
| Warning   | Recoverable errors.                                     | DB connection problem
| 0         | DEFAULT. Important state messages that won't repeat.    | Server started on 0.0.0.0
| 1         | Summarized info per request.                            | Sync for cluster completed. 
| 2         | Detailed of useful to debug individual requests.         |


## Local Development

1. Setup development `make setup`
2. Run `make setup-dev` and follow instructions in the output.
3. Run tests `make tests`
4. Run locally `make run`

Explore other supported tasks with `make help`.

## Unit Test

Unit tests mock the pgx connection object. More info: https://github.com/driftprogramming/pgxpoolmock


## Scale Test

Prerequisites: 

You must have **python**, **pip** and **locust** installed

*  Download latest version of python: https://www.python.org/downloads/
*  Install pip: https://pip.pypa.io/en/stable/installation/
*  Install locust  `pip install locust`
 
### Running Locust

1. Once we have the search indexer running with steps above, navigate to the **test** folder and run the following command:
`locust -f locust-clusters.py`

2. Follow the url provided (http://localhost:8089) and input load parameters in ui prompt.
Alternatively, we can just use this one line command instead of the UI for more automatated flow:
`locust -f locust-clusters.py --headless --users 10 --spawn-rate 5 -H https://localhost:3010`

For more on locust visit https://docs.locust.io/en/stable/index.html

Rebuild Date: 2022-09-26
