package main

import "receptify/database"

type OpenRouterResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type Job struct {
	ID     string           `json:"id"`
	Status string           `json:"status"`
	Result *database.Recept `json:"result,omitempty"` // Ha kész, itt a recept
	Error  string           `json:"error,omitempty"`
}

// 2. Represents a single piece of content (either Text or Image)
type ContentPart struct {
	Type     string    `json:"type"`                // "text" or "image_url"
	Text     string    `json:"text,omitempty"`      // Only used if Type is "text"
	ImageURL *ImageURL `json:"image_url,omitempty"` // Only used if Type is "image_url"
}

// 3. Represents the User Message
type Message struct {
	Role    string        `json:"role"`
	Content []ContentPart `json:"content"`
}

// 4. The top-level Request Object
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}
