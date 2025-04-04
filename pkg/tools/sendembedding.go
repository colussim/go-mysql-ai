package tools

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

func findSimilarMedicationsEM() {

	model := Config.Model.Name
	fmt.Println(model)
}
