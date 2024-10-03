
# To run the project please open a terminal and use these commands 

### create a new repository that would contain the project
```bash
mkdir bahas_work
```

### clone the project from my github repository
```bash
git clone https://github.com/bahagh/Go-project.git
```
### access the project
```bash
cd Go-project
```
### access the monitoring folder to run the database , prometheus and grafana containers
```bash
 cd monitoring
```

### run and build the containers in the background
```bash
docker-compose up -d --build
```

### now the containers are running , we'll start the producer service (if you are not using windows cmd or power shell , use "cd ../producer" then "go run main.go" )

```bash
start cmd /k "cd ../producer && echo Starting Producer Service... && go run main.go"
```


### now the containers are running , we'll start the consumer service (if you are not using windows cmd or power shell , use "cd ../consumer" then "go run main.go" )

```bash
start cmd /k "cd ../consumer && echo Starting Producer Service... && go run main.go"
```

