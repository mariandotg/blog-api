package hello

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"os"

	"gopkg.in/yaml.v2"
	"github.com/joho/godotenv"
	"github.com/gin-gonic/gin"
)

type TreeItem struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Sha  string `json:"sha"`
	Url  string `json:"url"`
}

type GitHubResponse struct {
	Sha  string     `json:"sha"`
	Tree []TreeItem `json:"tree"`
}

type PreviewPost struct {
	Title 			string `json:"title"`
	Description		string `json:"description"`
	Date			string `json:"date"`
	Slug			string `json:"slug"`
}

type PostFrontmatter struct {
	Title 		string `yaml:"title" json:"title"`
	Description string `yaml:"description" json:"description"`
	Date		string `yaml:"date" json:"date"`
	Slug    	string `yaml:"slug" json:"slug"`
}

type Post struct {
	Frontmatter 	PostFrontmatter `json:"frontmatter"`
	Content			string `json:"content"`
}

func goDotEnvVariable(key string) string {
	// load .env file
	err := godotenv.Load(".env")

	if err != nil {
		return fmt.Sprintf("Error loading .env file")
	}

	return os.Getenv(key)
}

func parseFrontmatter(content string) (PostFrontmatter, string, error) {
	var fm PostFrontmatter

	// Verificamos si el contenido empieza con '---' para detectar el frontmatter
	if strings.HasPrefix(content, "---") {
		// Buscamos el final del frontmatter
		parts := strings.SplitN(content, "---", 3)
		if len(parts) < 3 {
			return fm, "", fmt.Errorf("no se encontró el frontmatter correctamente delimitado")
		}

		// El frontmatter es el segundo elemento del split (después del primer '---')
		frontmatterContent := parts[1]
		// El contenido del post es el tercer elemento (después del segundo '---')
		postContent := parts[2]

		// Parseamos el frontmatter como YAML
		err := yaml.Unmarshal([]byte(frontmatterContent), &fm)
		if err != nil {
			return fm, "", fmt.Errorf("error parseando el frontmatter: %v", err)
		}

		return fm, postContent, nil
	}

	// Si no hay frontmatter, simplemente devolvemos el contenido original
	return fm, content, nil
}

var secretKey string

func authMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Obtener el valor del header Authorization
        authHeader := c.GetHeader("Authorization")
        
        // Eliminar espacios en blanco antes o después del valor
        authHeader = strings.TrimSpace(authHeader)

        // Si el header Authorization está vacío o no coincide con el secreto
        if authHeader == "" {
            c.JSON(http.StatusUnauthorized, gin.H{
                "error": "Authorization header vacío o faltante",
            })
            c.Abort() // Finaliza la solicitud
            return
        }

        // Si es un formato tipo Bearer, separa el valor real
        if strings.HasPrefix(authHeader, "Bearer ") {
            authHeader = strings.TrimPrefix(authHeader, "Bearer ")
        }

        // Verificamos si el secreto es válido
        if authHeader != secretKey {
            c.JSON(http.StatusUnauthorized, gin.H{
                "error": "Acceso no autorizado, secreto incorrecto",
            })
            c.Abort() // Finaliza la solicitud
            return
        }

        // Si el secreto es válido, continúa con el siguiente handler
        c.Next()
    }
}

func init() {
	secretKey = goDotEnvVariable("API_SECRET")

	if secretKey == "" {
        panic("API_SECRET no configurado")
    }

	router := gin.Default()
	router.Use(authMiddleware())
	router.GET("/posts", getAllPostsHandler)
	router.GET("/posts/:slug", getSinglePostHandler)
	router.Run()
}

func getPost(slug, locale string) (Post, error) {
	rootURL := fmt.Sprintf("https://raw.githubusercontent.com/mariandotg/blog/main/posts/%s/%s.mdx", slug, locale)
	authToken := goDotEnvVariable("GITHUB_AUTH_TOKEN")
	client := &http.Client{}

	req, err := http.NewRequest("GET", rootURL, nil)
	if err != nil {
		// handle error
		return Post{}, fmt.Errorf("error creando req: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", authToken))

	resp, err := client.Do(req)
	if err != nil {
		return Post{}, fmt.Errorf("error haciendo fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Post{}, fmt.Errorf("error leyendo cuerpo: %w", err)
	}

	frontmatter, postContent, err := parseFrontmatter(string(body))	
	if err != nil {
		return Post{}, fmt.Errorf("error parseando frontmatter %w", err)
	}

	cleanContent := strings.TrimSpace(postContent)
	
	return Post{ Frontmatter: frontmatter, Content: cleanContent}, nil
}

func getAllPosts() ([]byte, error) {
	rootURL := "https://api.github.com/repos/mariandotg/blog/git/trees/main?recursive=1"
	authToken := goDotEnvVariable("GITHUB_AUTH_TOKEN")
	client := &http.Client{}

	// Hacemos la petición GET a la URL
	req, err := http.NewRequest("GET", rootURL, nil)
	if err != nil {
		// handle error
		return nil, fmt.Errorf("error creando req: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", authToken))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error haciendo fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error leyendo cuerpo: %w", err)
	}

	return body, nil
}

func getSinglePostHandler(c *gin.Context) {
	locale := c.DefaultQuery("locale", "en")
	slug := c.Param("slug")

	post, err := getPost(slug, locale)
	if err != nil {
		// En caso de error, respondemos con un mensaje adecuado
		c.String(http.StatusInternalServerError, "ERROR: %v", err)
		return
	}
	// Respondemos con el contenido del archivo
	c.JSON(http.StatusOK, post)
}

func getAllPostsHandler(c *gin.Context) {
	locale := c.DefaultQuery("locale", "en")	

	// Leemos el body de la respuesta
	body, err := getAllPosts() 
	if err != nil {
		c.String(http.StatusInternalServerError, "ERROR:  %v", err)
		return
	}

	// Parseamos el JSON en un struct para obtener los elementos
	var data struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		c.String(http.StatusInternalServerError, "ERROR PARSEANDO JSON")
		return
	}

	// Creamos un slice para almacenar los resultados de los PreviewPosts
	var previewPosts []PreviewPost

	// Recorremos los archivos del árbol y buscamos los que coincidan con el locale
	for _, item := range data.Tree {
		// Filtramos solo los archivos que tengan el locale correcto y terminen en '.md'
		if strings.HasSuffix(item.Path, fmt.Sprintf("/%s.mdx", locale)) {
			// Extraemos el slug del path (sería algo como 'posts/post-1/en.md' => 'post-1')
			slug := strings.Split(item.Path, "/")[1]

			// Obtenemos el contenido del post llamando a getPost
			post, err := getPost(slug, locale)
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
			

			// Agregamos el post al array de resultados
			previewPosts = append(previewPosts, PreviewPost{
				Title: 			post.Frontmatter.Title,
				Description:	post.Frontmatter.Description,
				Date: 			post.Frontmatter.Date,
				Slug:    		post.Frontmatter.Slug,
			})
		}
	}

	// Devolvemos el array de PreviewPosts como JSON
	c.JSON(http.StatusOK, previewPosts)
}
//https://api.github.com/repos/gitdagray/test-blogposts/git/trees/main?recursive=1
