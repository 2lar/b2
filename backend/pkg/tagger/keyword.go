package tagger

import (
	"context"
	"regexp"
	"strings"
)

// keywordTagger implements the Tagger interface using keyword extraction and semantic grouping.
type keywordTagger struct {
	maxTags    int
	stopWords  map[string]bool
	categories map[string][]string // Maps keywords to category tags
}

// NewKeywordTagger creates a new keyword-based tagger with semantic grouping.
func NewKeywordTagger(config Config) Tagger {
	tagger := &keywordTagger{
		maxTags:   config.MaxTags,
		stopWords: getStopWords(),
		categories: getSemanticCategories(),
	}
	
	return tagger
}

// GenerateTags extracts keywords and groups them into semantic categories.
func (k *keywordTagger) GenerateTags(ctx context.Context, content string) ([]string, error) {
	// Extract keywords using similar logic to the existing memory service
	keywords := k.extractKeywords(content)
	
	// Group keywords into semantic categories
	categories := k.groupIntoCategories(keywords)
	
	// Add significant keywords as tags if they don't map to categories
	significantKeywords := k.getSignificantKeywords(keywords)
	
	// Combine categories and significant keywords
	allTags := append(categories, significantKeywords...)
	
	// Remove duplicates and limit results
	tags := k.deduplicateAndLimit(allTags)
	
	return tags, nil
}

// HealthCheck verifies that the keyword tagger is functioning properly.
func (k *keywordTagger) HealthCheck(ctx context.Context) error {
	// Test with a simple string to ensure basic functionality
	_, err := k.GenerateTags(ctx, "test content for health check")
	return err
}

// extractKeywords extracts meaningful keywords from text content.
func (k *keywordTagger) extractKeywords(content string) []string {
	// Convert to lowercase and remove punctuation
	content = strings.ToLower(content)
	reg := regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	content = reg.ReplaceAllString(content, "")
	words := strings.Fields(content)
	
	// Filter out stop words and short words
	var keywords []string
	wordFreq := make(map[string]int)
	
	for _, word := range words {
		if !k.stopWords[word] && len(word) > 2 {
			wordFreq[word]++
			if wordFreq[word] == 1 { // Only add unique words
				keywords = append(keywords, word)
			}
		}
	}
	
	return keywords
}

// groupIntoCategories maps keywords to semantic categories.
func (k *keywordTagger) groupIntoCategories(keywords []string) []string {
	categoryMatches := make(map[string]bool)
	
	for _, keyword := range keywords {
		for category, categoryKeywords := range k.categories {
			for _, catKeyword := range categoryKeywords {
				if strings.Contains(keyword, catKeyword) || strings.Contains(catKeyword, keyword) {
					categoryMatches[category] = true
					break
				}
			}
		}
	}
	
	var categories []string
	for category := range categoryMatches {
		categories = append(categories, category)
	}
	
	return categories
}

// getSignificantKeywords returns keywords that are likely to be meaningful tags.
func (k *keywordTagger) getSignificantKeywords(keywords []string) []string {
	var significant []string
	
	for _, keyword := range keywords {
		// Include longer words and technical terms
		if len(keyword) >= 4 && !k.isCommonWord(keyword) {
			significant = append(significant, keyword)
		}
	}
	
	return significant
}

// isCommonWord checks if a word is too common to be a meaningful tag.
func (k *keywordTagger) isCommonWord(word string) bool {
	commonWords := map[string]bool{
		"time": true, "work": true, "want": true, "need": true,
		"make": true, "take": true, "come": true, "good": true,
		"right": true, "back": true, "think": true, "know": true,
		"people": true, "thing": true, "something": true, "nothing": true,
	}
	return commonWords[word]
}

// deduplicateAndLimit removes duplicates and limits the number of tags.
func (k *keywordTagger) deduplicateAndLimit(tags []string) []string {
	seen := make(map[string]bool)
	var result []string
	
	for _, tag := range tags {
		if !seen[tag] && isValidTag(tag) {
			seen[tag] = true
			result = append(result, tag)
			
			// Respect max tags limit
			if k.maxTags > 0 && len(result) >= k.maxTags {
				break
			}
		}
	}
	
	return result
}

// getStopWords returns a comprehensive list of stop words.
func getStopWords() map[string]bool {
	return map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "from": true, "up": true, "about": true,
		"into": true, "through": true, "during": true, "before": true, "after": true,
		"above": true, "below": true, "between": true, "under": true,
		"again": true, "further": true, "then": true, "once": true,
		"is": true, "am": true, "are": true, "was": true, "were": true,
		"be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true, "did": true,
		"will": true, "would": true, "should": true, "could": true, "ought": true,
		"i": true, "me": true, "my": true, "myself": true,
		"we": true, "our": true, "ours": true, "ourselves": true,
		"you": true, "your": true, "yours": true, "yourself": true, "yourselves": true,
		"he": true, "him": true, "his": true, "himself": true,
		"she": true, "her": true, "hers": true, "herself": true,
		"it": true, "its": true, "itself": true,
		"they": true, "them": true, "their": true, "theirs": true, "themselves": true,
		"what": true, "which": true, "who": true, "whom": true,
		"this": true, "that": true, "these": true, "those": true,
		"as": true, "if": true, "each": true, "how": true, "than": true,
		"too": true, "very": true, "can": true, "just": true, "also": true,
	}
}

// getSemanticCategories returns a mapping of semantic categories to keywords.
func getSemanticCategories() map[string][]string {
	return map[string][]string{
		"technology": {"computer", "software", "app", "digital", "online", "internet", "web", "tech", "data", "algorithm", "programming", "code", "system", "platform", "device", "mobile"},
		"business": {"company", "business", "market", "customer", "revenue", "profit", "strategy", "management", "sales", "finance", "budget", "investment", "startup", "enterprise"},
		"health": {"health", "fitness", "exercise", "diet", "nutrition", "medical", "doctor", "hospital", "wellness", "mental", "physical", "therapy", "treatment"},
		"education": {"learn", "education", "school", "university", "study", "course", "training", "knowledge", "skill", "teacher", "student", "research", "academic"},
		"lifestyle": {"lifestyle", "daily", "routine", "habits", "personal", "family", "home", "travel", "hobby", "entertainment", "leisure", "social"},
		"work": {"job", "career", "professional", "workplace", "office", "project", "task", "meeting", "deadline", "team", "colleague", "productivity"},
		"finance": {"money", "financial", "bank", "credit", "loan", "savings", "investment", "insurance", "tax", "budget", "expense", "income"},
		"creative": {"art", "design", "creative", "music", "writing", "photography", "video", "drawing", "painting", "craft", "aesthetic"},
		"food": {"food", "cooking", "recipe", "restaurant", "meal", "dinner", "lunch", "breakfast", "cuisine", "ingredient", "nutrition", "kitchen"},
		"sports": {"sport", "game", "team", "player", "competition", "training", "fitness", "athletic", "exercise", "match", "tournament"},
		"science": {"science", "research", "experiment", "discovery", "theory", "analysis", "study", "data", "evidence", "hypothesis", "scientific"},
		"relationships": {"relationship", "friend", "family", "partner", "love", "social", "communication", "trust", "support", "connection"},
	}
}