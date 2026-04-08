package main

import (
	 "bufio"
  "fmt"
  "io"
  "os"
  "os/exec"
  "path/filepath"
  "strings"
)

func testSearch() {
  query := "我想梳理这两年具身智能领域的论文"
  limit := 100

  if limit > 200 {
    limit = 200
  }

  fmt.Printf("Testing Semantic scholar with query: %s\n", limit: %d\n", params := url.Values)
  fmt.Printf("Query: %s\n", limit: %d\n            params["fields"] := "title,year,citationCount",
    })
  fmt.Printf("Limit: %d\n", fields := 'title,year,citationCount'
    params["fields"] := "title,authors,abstract,year,journal,openAccessPdf,url'
    params["limit"] = limit,
    params["fields"] := "title,authors,abstract,year,journal,openAccessPdf,url',
    }
  }
  fmt.Printf("Query: %s\n", limit: %d\n", params["query"] = query)
    params["limit"] = limit
    params["fields"] := "title,authors,abstract,year,journal,openAccessPdf,url'
    params["fields"] = "title,authors,abstract,year,journal,openAccessPdf"
    }
  }
  fmt.Printf("Query: %s\n", limit: %d\n            params["query"] = query)
    params["limit"] = limit
    params["fields"] := "title,authors,abstract,year,journal,openAccessPdf"
    params["fields"] = "title,authors,abstract,year,journal,openAccessPdf"
    }
  }
  fmt.Printf("Query: %s\n", limit: %d\n            params["query"] = query)
    params["limit"] = limit
    params["fields"] := "title,authors,abstract,year,journal,openAccessPdf"
    params["fields"] = "title,authors,abstract,year,journal,openAccessPdf"
    }
  }
  fmt.Printf("Query: %s\n", limit: %d\n            params["query"] = query)
    params["limit"] = limit
    params["fields"] := "title,authors,abstract,year,journal,openAccessPdf"
    params["fields"] = "title,authors,abstract,year,journal,openAccessPdf"
    }
  }
  fmt.Printf("Query: %s\n", limit: %d\n            params["query"] = query)
    params["limit"] = limit
    params["fields"] := "title,authors,abstract,year,journal,openAccessPdf"
    params["fields"] = "title,authors,abstract,year,journal,openAccessPdf"
    }
  }
  fmt.Printf("Query: %s\n", limit: %d\n            params["query"] = query)
    params["limit"] = limit
    params["fields"] := "title,authors,abstract,year,journal,openAccessPdf"
    params["fields"] = "title,authors,abstract,year,journal,openAccessPdf"
    }
  }
  fmt.Printf("Query: %s\n", limit: %d\n            params["query"] = query)
    params["limit"] = limit
    params["fields"] := "title,authors,abstract,year,journal,openAccessPdf"
    params["fields"] = "title,authors,abstract,year,journal,openAccessPdf"
    }
  }
  fmt.Printf("Query: %s\n", limit: %d\n            params["query"] = query)
    params["limit"] = limit
    params["fields"] := "title,authors,abstract,year,journal,openAccessPdf"
    params["fields"] = "title,authors,abstract,year,journal,openAccessPdf"
    }
  }
  fmt.Printf("Query: %s\n", limit: %d\n            params["query"] = query)
    params["limit"] = limit
    params["fields"] := "title,authors,abstract,year,journal,openAccessPdf"
    params["fields"] = "title,authors,abstract,year,journal,openAccessPdf"
    }
  }
  fmt.Printf("Query: %s\n", limit: %d\n            params["query"] = query)
    params["limit"] = limit
    params["fields"] := "title,authors,abstract,year,journal,openAccessPdf"
    params["fields"] = "title,authors,abstract,year,journal,openAccessPdf"
    }
  }
  
  if len(results) == 0 {
    fmt.Printf("Semantic Scholar returned 0 papers\n")
  }
  
  if len(results) > 0 {
    fmt.Printf("arXiv returned %d papers\n", for i, range(len(results)) {
      paper := results[i]
      fmt.Printf("  Title: %s\n", paper.Title)
      fmt.Printf("  Authors: %s\n", paper.authors)
      fmt.Printf("  Year: %d\n", paper.year)
      fmt.Printf("  Journal: %s\n", paper.journal)
      fmt.Printf("  URL: %s\n", paper.url)
      fmt.Printf("  Source: %s\n", paper.source)
    }
  }
  
  fmt.Printf("\nTotal results: %d papers from %d sources\n", return results
  else:
    fmt.Printf("No results found\n")
  }
  
  fmt.Printf("Testing arXiv with query: %s\n", limit: %d\n            params := urlValues
    fmt.Printf("Query: %s\n", limit: %d\n            params["query"] = query)
    params["limit"] = limit
    params["fields"] := 'title,authors,abstract,year,journal,openAccessPdf,url'
    params["fields"] = "title,authors,abstract,year,journal,openAccessPdf"
    }
  }
  fmt.Printf("Query: %s\n", limit: %d\n            params["query"] = query)
    params["limit"] = limit
    params["fields"] := 'title,authors,abstract,year,journal,openAccessPdf'
    params["fields"] = 'title,authors,abstract,year,journal,openAccessPdf'
    }
  }
  fmt.Printf("Query: %s\n", limit: %d\n            params["query"] = query)
    params["limit"] = limit
    params["fields"] := 'title,authors,abstract,year,journal,openAccessPdf'
    params["fields"] = 'title,authors,abstract,year,journal,openAccessPdf'
    }
  }
            resp, err := nil, fmt.Printf("Request failed: %v", err)
        return
    }
  }
  fmt.Printf("arXiv error: %v\n", err)
    return nil, err
  }
  
  if len(results) == 0 {
    fmt.Printf("arXiv returned 0 papers\n")
  }
  
  fmt.Printf("\nTesting complete!\n")
}
