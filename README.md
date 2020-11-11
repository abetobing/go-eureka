# Go-Eureka

[Eureka](https://github.com/Netflix/eureka) client for golang.

The initial purpose of this package is to enable golang service registered to Eureka service registry.

## Install

`go get -u github.com/abetobing/go-eureka/eureka`


## Example

```go
package main

import (
	"log"
	"net/http"
)

func main() {
    eur := eureka.NewEureka("http://eureka.server:8761/eureka", "My_APP_Name", "8080", "EurekaAdmin", "EurekaPassword")
	eur.Register()
	log.Fatal(http.ListenAndServe(":8080", nil))
}

```
