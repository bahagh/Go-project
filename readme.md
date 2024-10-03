
### inside the root folder open a terminal and follow this commands to run the project

```bash
 cd monitoring
```
```bash
docker-compose up --build
```
### in another terminal window :

```bash
cd producer
```

```bash
go run main.go
```

### in another terminal window :

```bash
cd consumer
```

```bash
go run main.go
```
