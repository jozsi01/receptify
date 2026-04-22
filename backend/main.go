package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"receptify/database"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

var receptek []database.Recept
var jobStore = sync.Map{}

type ReceptListPageData struct {
	Recipes               []database.Recept
	SelectedCategory      string
	AvailableIngredients  []string
	SelectedIngredientMap map[string]bool
	HasIngredientFilter   bool
}

func normalizeIngredientName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func buildAvailableIngredients(recepts []database.Recept) []string {
	seen := map[string]string{}
	for _, recept := range recepts {
		for ingredient := range recept.Ingridients {
			normalized := normalizeIngredientName(ingredient)
			if normalized == "" {
				continue
			}
			if _, exists := seen[normalized]; !exists {
				seen[normalized] = strings.TrimSpace(ingredient)
			}
		}
	}

	result := make([]string, 0, len(seen))
	for _, ingredient := range seen {
		result = append(result, ingredient)
	}
	sort.Strings(result)
	return result
}

func filterReceptsByIngredients(recepts []database.Recept, selectedIngredients []string) []database.Recept {
	if len(selectedIngredients) == 0 {
		return recepts
	}

	normalizedSelected := make([]string, 0, len(selectedIngredients))
	for _, ingredient := range selectedIngredients {
		normalized := normalizeIngredientName(ingredient)
		if normalized != "" {
			normalizedSelected = append(normalizedSelected, normalized)
		}
	}
	if len(normalizedSelected) == 0 {
		return recepts
	}

	filtered := make([]database.Recept, 0, len(recepts))
	for _, recept := range recepts {
		recipeIngredients := map[string]bool{}
		for ingredient := range recept.Ingridients {
			recipeIngredients[normalizeIngredientName(ingredient)] = true
		}

		matchesAll := true
		for _, selected := range normalizedSelected {
			if !recipeIngredients[selected] {
				matchesAll = false
				break
			}
		}

		if matchesAll {
			filtered = append(filtered, recept)
		}
	}

	return filtered
}

func handlePrompt(w http.ResponseWriter, req *http.Request) {
	req.ParseMultipartForm(10 << 20)
	encodedImages := make([]string, 0)
	for _, files := range req.MultipartForm.File {
		for _, fileHeader := range files {
			mimeType := fileHeader.Header.Get("Content-Type")
			if mimeType == "" {
				mimeType = "image/jpeg" // Fallback if header is missing
			}
			file, err := fileHeader.Open()
			file.Close()
			if err != nil {
				panic(err)
			}
			filebytes, err := io.ReadAll(file)
			if err != nil {
				panic(err)
			}
			base64Raw := base64.StdEncoding.EncodeToString(filebytes)
			encodedImage := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Raw)
			encodedImages = append(encodedImages, encodedImage)

		}
	}
	jobID := fmt.Sprintf("job_%d", time.Now().UnixNano())
	jobStore.Store(jobID, Job{
		ID:     jobID,
		Status: "pending",
	})
	go sendPrompt(jobID, encodedImages)
	fmt.Println(jobID)

	json.NewEncoder(w).Encode(map[string]string{"job_id": jobID})
}

func createRequestBody(encodedImages []string) string {
	model := "google/gemma-3-27b-it:free"
	prompt := "Te egy szakács segéd vagy. Csatoltam 1 vagy több képet egy receptről. Elemezd a képeket, és a válaszod szigorúan csak valid JSON legyen, semmi más szöveg vagy markdown. A JSON struktúra pontosan ez legyen: {\"recept_neve\":\"string\",\"kategoria\":\"dessert|main|soup|appetizers\",\"hozzavalok\":{\"hozzavalo\":\"mennyiseg\"},\"elkeszites\":\"string\"}. A kategória mező kötelező, és csak az alábbi négy érték egyike lehet: dessert, main, soup, appetizers. Te döntsd el a legjobb kategóriát a kép alapján. Ha több kép van, egyetlen receptté fűzd össze az információkat. Az elkészítés legyen egy hosszabb, összefüggő string. Ha egy hozzávalónál nincs mennyiség, az érték legyen ízlés szerint."
	content := make([]ContentPart, 1)
	content[0] = ContentPart{
		Type: "text",
		Text: prompt,
	}
	for _, image := range encodedImages {
		content = append(content, ContentPart{
			Type:     "image_url",
			ImageURL: &ImageURL{URL: image},
		})
	}

	request := ChatRequest{
		Model: model,
		Messages: []Message{
			{
				Role:    "user",
				Content: content,
			},
		},
		Stream: false,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		fmt.Printf("Parsing request body err: %s", err)
	}
	return string(jsonData)
}
func sendPrompt(jobID string, images []string) {

	apiKey := os.Getenv("API_KEY")
	client := http.Client{Timeout: time.Duration(120) * time.Second}

	jsonPrompt := createRequestBody(images)
	jsonData := bytes.NewBuffer([]byte(jsonPrompt))
	httpreq, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", jsonData)
	if err != nil {
		fmt.Printf("error %s", err)
	}
	httpreq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	httpreq.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(httpreq)
	if err != nil {
		fmt.Printf("error %s", err)

	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	fmt.Printf("raw response: %s\n", body)
	var respPrompt OpenRouterResponse
	err = json.Unmarshal(body, &respPrompt)
	if err != nil || len(respPrompt.Choices) == 0 {
		jobStore.Store(jobID, Job{
			ID:     jobID,
			Status: "error",
			Result: &database.Recept{},
		})
		return
	}
	fmt.Printf("oprouter resp: %+v", respPrompt)
	var recept database.Recept

	elso := strings.Index(respPrompt.Choices[0].Message.Content, "{")
	uccso := strings.LastIndex(respPrompt.Choices[0].Message.Content, "}")
	formattedContent := respPrompt.Choices[0].Message.Content[elso : uccso+1]
	err = json.Unmarshal([]byte(formattedContent), &recept)
	if err != nil {
		fmt.Printf("Baj volta  REcept parsolásnál: %s\n", err)
	}
	jobStore.Store(jobID, Job{
		ID:     jobID,
		Status: "done",
		Result: &recept,
	})
	_, err = database.SaveRecept(recept)
	if err != nil {
		panic(err)
	}
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("id")

	data, ok := jobStore.Load(jobID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func handleMainView(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/index.html")
}
func handleReceptekView(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("./templates/recept.html")
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	selectedCategory := database.NormalizeCategory(r.URL.Query().Get("category"))
	selectedIngredients := r.URL.Query()["ingredient"]
	receptek, err = database.GetAllRecepts(selectedCategory)
	if err != nil {
		panic(err)
	}

	availableIngredients := buildAvailableIngredients(receptek)
	receptek = filterReceptsByIngredients(receptek, selectedIngredients)

	selectedIngredientMap := map[string]bool{}
	for _, ingredient := range selectedIngredients {
		trimmed := strings.TrimSpace(ingredient)
		if trimmed != "" {
			selectedIngredientMap[trimmed] = true
		}
	}

	pageData := ReceptListPageData{
		Recipes:               receptek,
		SelectedCategory:      selectedCategory,
		AvailableIngredients:  availableIngredients,
		SelectedIngredientMap: selectedIngredientMap,
		HasIngredientFilter:   len(selectedIngredientMap) > 0,
	}
	err = tmpl.Execute(w, pageData)
	if err != nil {
		fmt.Printf("Template parrsing err: %s\n", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
}

func handleCookingView(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := database.GetReceptByID(id)
	fmt.Printf("res: %v\n", result)
	if err != nil {
		fmt.Println(err)
		http.NotFound(w, r)
		return
	}
	tmpl, err := template.ParseFiles("./templates/cooking.html")
	if err != nil {
		http.Error(w, "Hiba a template betöltésekor", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, result)

}
func handleComment(w http.ResponseWriter, r *http.Request) {
	receptId := r.PathValue("id")

	// 1. Multipart form feldolgozása (max 32 MB memória)
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, "Túl nagy adatmennyiség", http.StatusBadRequest)
		return
	}

	starsStr := r.FormValue("stars")
	commentText := r.FormValue("comment")
	stars, _ := strconv.Atoi(starsStr)
	fmt.Printf("Commenttext: %s és stars: %d\n", commentText, stars)
	// Ebbe gyűjtjük a fájlneveket
	var savedFilenames []string

	// 2. A "images" kulcs alatt lévő fájlok lekérése
	files := r.MultipartForm.File["images"]
	fmt.Printf("Ennyi kepet csatoltak: %d\n", len(files))
	// Ciklus a feltöltött fájlokon
	for _, fileHeader := range files {
		// Fájl megnyitása
		file, err := fileHeader.Open()
		if err != nil {
			fmt.Println("Hiba a fájl megnyitásakor:", err)
			continue
		}
		defer file.Close()

		// Egyedi fájlnév: időbélyeg + eredeti név
		// (A time.Now().UnixNano() biztosítja, hogy ne legyenek azonos nevűek)
		filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), fileHeader.Filename)

		// Mappa és útvonal (static/uploads mappának léteznie kell!)
		os.MkdirAll(filepath.Join("static", "uploads"), os.ModePerm)
		dstPath := filepath.Join("static", "uploads", filename)
		// Üres fájl létrehozása a lemezen
		dst, err := os.Create(dstPath)
		if err != nil {
			fmt.Println("Hiba a fájl létrehozásakor:", err)
			continue
		}

		// Tartalom átmásolása
		if _, err := io.Copy(dst, file); err != nil {
			fmt.Println("Hiba a másoláskor:", err)
			dst.Close()
			continue
		}
		dst.Close()

		// Ha sikeres, hozzáadjuk a listához
		savedFilenames = append(savedFilenames, filename)
	}

	// 3. Struct összeállítása a fájlnevek listájával
	ujKomment := database.Comment{
		ID:      strconv.FormatInt(time.Now().UnixNano(), 10),
		Stars:   stars,
		Comment: commentText,
		Images:  savedFilenames, // Itt adjuk át a tömböt
	}

	// 4. Mentés az adatbázisba
	if err := database.AddCommentToRecept(receptId, ujKomment); err != nil {
		http.Error(w, "Adatbázis hiba: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Siker"))
}

func handleCommentView(w http.ResponseWriter, r *http.Request) {
	receptId := r.PathValue("id")
	result, err := database.GetReceptByID(receptId)
	if err != nil {
		fmt.Println(err)
		http.NotFound(w, r)
		return
	}
	tmpl, err := template.ParseFiles("./templates/komment.html")
	if err != nil {
		http.Error(w, "Hiba a template betöltésekor", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, result)

}

func handleDeleteRecept(w http.ResponseWriter, r *http.Request) {
	receptID := r.PathValue("id")
	if strings.TrimSpace(receptID) == "" {
		http.Error(w, "Hianyzo recept azonosito", http.StatusBadRequest)
		return
	}

	deleted, err := database.DeleteReceptByID(receptID)
	if err != nil {
		http.Error(w, "Adatbazis hiba", http.StatusInternalServerError)
		return
	}
	if !deleted {
		http.Error(w, "A recept nem talalhato", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func main() {
	godotenv.Load()
	database.ConnectDatabase()

	http.HandleFunc("/prompt", handlePrompt)
	http.HandleFunc("GET /cooking/{id}", handleCookingView)
	http.HandleFunc("POST /comment/{id}", handleComment)
	http.HandleFunc("GET /receptek/{id}/kommentek", handleCommentView)
	http.HandleFunc("DELETE /receptek/{id}", handleDeleteRecept)
	http.HandleFunc("/", handleMainView)
	http.HandleFunc("/receptek", handleReceptekView)
	http.HandleFunc("/status", handleStatus)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.ListenAndServe(":8080", nil)

}
