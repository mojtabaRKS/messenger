package paginator

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

type Paginate struct {
	From, Size, Page int
}

func New(c *gin.Context) Paginate {
	sizeStr := c.DefaultQuery("page_size", "10")
	pageStr := c.DefaultQuery("page", "1")

	size, _ := strconv.Atoi(sizeStr)
	page, _ := strconv.Atoi(pageStr)

	from := 0
	if page > 0 {
		from = (page - 1) * size
	}

	return Paginate{
		From: from,
		Size: size,
		Page: page,
	}
}
