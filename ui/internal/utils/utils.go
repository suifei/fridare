package utils

import (
	"crypto/md5"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// GetExecutableDir 获取可执行文件所在目录
func GetExecutableDir() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(executable), nil
}

// GetConfigDir 获取配置目录
func GetConfigDir() (string, error) {
	var configDir string

	switch runtime.GOOS {
	case "windows":
		configDir = os.Getenv("APPDATA")
		if configDir == "" {
			configDir = os.Getenv("USERPROFILE")
		}
	case "darwin":
		configDir = os.Getenv("HOME")
		if configDir != "" {
			configDir = filepath.Join(configDir, "Library", "Application Support")
		}
	default: // linux and others
		configDir = os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			configDir = os.Getenv("HOME")
			if configDir != "" {
				configDir = filepath.Join(configDir, ".config")
			}
		}
	}

	if configDir == "" {
		return "", fmt.Errorf("无法确定配置目录")
	}

	return filepath.Join(configDir, "fridare"), nil
}

// EnsureDir 确保目录存在
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// FileExists 检查文件是否存在
func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

// DirExists 检查目录是否存在
func DirExists(dirname string) bool {
	info, err := os.Stat(dirname)
	return err == nil && info.IsDir()
}

// CalculateMD5 计算文件MD5
func CalculateMD5(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// GetFileSize 获取文件大小
func GetFileSize(filename string) (int64, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// FormatBytes 格式化字节大小
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// CleanPath 清理路径
func CleanPath(path string) string {
	// 替换反斜杠为正斜杠（Windows兼容性）
	path = strings.ReplaceAll(path, "\\", "/")
	// 清理路径
	return filepath.Clean(path)
}

// GetRelativePath 获取相对路径
func GetRelativePath(basepath, targetpath string) (string, error) {
	return filepath.Rel(basepath, targetpath)
}

// SanitizeFilename 清理文件名（移除非法字符）
func SanitizeFilename(filename string) string {
	// Windows禁用的字符
	invalidChars := []string{"<", ">", ":", "\"", "|", "?", "*", "/", "\\"}

	for _, char := range invalidChars {
		filename = strings.ReplaceAll(filename, char, "_")
	}

	// 移除控制字符
	var result strings.Builder
	for _, r := range filename {
		if r >= 32 && r != 127 {
			result.WriteRune(r)
		}
	}

	return strings.TrimSpace(result.String())
}

// CopyFile 复制文件
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// 确保目标目录存在
	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// MoveFile 移动文件
func MoveFile(src, dst string) error {
	// 确保目标目录存在
	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return err
	}

	// 先尝试重命名（如果在同一个分区）
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// 如果重命名失败，则复制后删除
	if err := CopyFile(src, dst); err != nil {
		return err
	}

	return os.Remove(src)
}

// BackupFile 备份文件
func BackupFile(filename string) error {
	backupName := filename + ".bak"
	return CopyFile(filename, backupName)
}

// IsValidHex 检查是否为有效的十六进制字符串
func IsValidHex(s string) bool {
	// 移除空格和常见分隔符
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, ":", "")

	// 检查长度是否为偶数
	if len(s)%2 != 0 {
		return false
	}

	// 检查是否只包含十六进制字符
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}

	return true
}

// CleanHex 清理十六进制字符串
func CleanHex(s string) string {
	// 移除空格和常见分隔符
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")

	return strings.ToUpper(s)
}

// GetSystemInfo 获取系统信息
func GetSystemInfo() map[string]string {
	return map[string]string{
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
		"version": runtime.Version(),
		"cpus":    fmt.Sprintf("%d", runtime.NumCPU()),
	}
}

// FindExecutable 查找可执行文件
func FindExecutable(name string) (string, error) {
	// 在 PATH 中查找
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}

	// 在当前目录查找
	if FileExists(name) {
		return filepath.Abs(name)
	}

	// 在可执行文件目录查找
	execDir, err := GetExecutableDir()
	if err == nil {
		execPath := filepath.Join(execDir, name)
		if FileExists(execPath) {
			return execPath, nil
		}
	}

	return "", fmt.Errorf("找不到可执行文件: %s", name)
}

// FetchRemoteText 获取远程文本内容(支持代理)
func FetchRemoteText(urlStr string, proxyURL string) (string, error) {
	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 如果proxyURL 传入为空，则尝试使用系统代理
	if proxyURL == "" {
		if proxyURL, err := http.ProxyFromEnvironment(&http.Request{URL: &url.URL{Host: urlStr}}); err == nil && proxyURL != nil {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
		}
	}
	// 如果提供了代理URL，设置代理
	resp, err := client.Get(urlStr)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("请求失败: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// TestProxy 测试代理连接
func TestProxy(proxyURL string, testURL string, timeout int) (bool, string, error) {
	// 如果没有指定测试URL，使用默认的
	if testURL == "" {
		testURL = "https://api.github.com/repos/frida/frida/releases/latest"
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	// 如果提供了代理URL，设置代理
	if proxyURL != "" {
		proxyParsed, err := url.Parse(proxyURL)
		if err != nil {
			return false, "代理URL格式错误", err
		}

		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyParsed),
		}
		client.Transport = transport
	}

	// 发送测试请求
	start := time.Now()
	resp, err := client.Get(testURL)
	elapsed := time.Since(start)

	if err != nil {
		return false, fmt.Sprintf("连接失败: %v", err), err
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode == http.StatusOK {
		return true, fmt.Sprintf("连接成功 (%dms)", elapsed.Milliseconds()), nil
	} else {
		return false, fmt.Sprintf("HTTP状态码: %d", resp.StatusCode), nil
	}
}

var (
	nameRandMu sync.Mutex
	nameRand   = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// GenerateRandomName 生成随机魔改名：恰好 5 个小写字母 a-z。
// 必须与 core.ValidateMagicName / hexreplace isStringAlpha 一致，禁止数字填充。
func GenerateRandomName() string {
	// 使用包级 RNG，避免紧循环中 UnixNano 相同导致 seed 重复
	nameRandMu.Lock()
	defer nameRandMu.Unlock()
	rng := nameRand

	const letters = "abcdefghijklmnopqrstuvwxyz"

	// 1字母单词（常用缩写）
	oneLetterWords := []string{
		"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m",
		"n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
	}

	// 2字母单词
	twoLetterWords := []string{
		"ai", "am", "an", "as", "at", "be", "by", "do", "go", "he", "hi",
		"if", "in", "is", "it", "me", "my", "no", "of", "ok", "on", "or",
		"so", "to", "up", "we", "db", "os", "ui", "io", "js", "py", "qt",
		"ax", "ox", "ex", "rx", "tx", "dx", "fx", "mx", "nx", "px", "zx",
	}

	// 3字母单词
	threeLetterWords := []string{
		"add", "and", "any", "app", "art", "bad", "big", "box", "bug", "bus",
		"buy", "can", "car", "cat", "cpu", "cry", "cut", "day", "die", "dig",
		"doc", "dog", "eat", "end", "eye", "far", "fix", "fly", "for", "fun",
		"get", "god", "gun", "guy", "has", "hit", "hot", "how", "ice", "job",
		"key", "led", "let", "log", "lot", "low", "man", "map", "max", "may",
		"net", "new", "now", "old", "one", "our", "out", "own", "pay", "pdf",
		"put", "ram", "raw", "red", "rom", "row", "run", "see", "set", "sex",
		"she", "six", "sql", "sun", "tax", "tea", "ten", "the", "top", "try",
		"two", "use", "via", "war", "way", "web", "who", "why", "win", "yes",
		"you", "zip", "api", "css", "dll", "exe", "gif", "jpg", "png", "xml",
	}

	// 4字母单词
	fourLetterWords := []string{
		"able", "back", "base", "best", "blue", "body", "book", "both", "call",
		"came", "case", "chat", "city", "code", "come", "copy", "core", "data",
		"date", "deal", "deep", "desk", "diff", "disk", "done", "door", "down",
		"draw", "drop", "each", "edit", "else", "even", "ever", "exit", "face",
		"fact", "fail", "fast", "file", "find", "fire", "flag", "flow", "form",
		"free", "from", "full", "game", "give", "good", "grab", "hand", "hard",
		"have", "head", "help", "here", "hide", "high", "hold", "home", "hope",
		"host", "hour", "http", "huge", "icon", "idea", "into", "item", "join",
		"jump", "just", "keep", "kind", "know", "last", "late", "left", "like",
		"line", "link", "list", "live", "load", "lock", "long", "look", "loop",
		"love", "made", "mail", "main", "make", "many", "mark", "math", "menu",
		"meta", "mind", "mode", "more", "most", "move", "much", "must", "name",
		"near", "need", "news", "next", "nice", "node", "note", "only", "open",
		"over", "page", "part", "pass", "path", "plan", "play", "plus", "port",
		"post", "pull", "push", "quit", "race", "read", "real", "rich", "room",
		"root", "rule", "same", "save", "scan", "seal", "seek", "seem", "self",
		"sell", "send", "show", "shut", "side", "sign", "size", "some", "sort",
		"spin", "stop", "sure", "swap", "take", "talk", "task", "team", "tell",
		"test", "text", "than", "that", "them", "then", "they", "this", "time",
		"tiny", "tool", "true", "turn", "type", "unit", "upon", "used", "user",
		"very", "view", "wait", "walk", "want", "what", "when", "with", "word",
		"work", "year", "your", "zero", "ajax", "bash", "boot", "bulk", "byte",
		"calc", "cash", "chef", "clip", "club", "cool", "ctrl", "curl", "demo",
		"draw", "dump", "echo", "exec", "font", "grep", "hash", "head", "heap",
		"http", "init", "java", "jpeg", "json", "kill", "lamp", "lens", "lint",
		"loop", "mask", "mega", "mono", "nano", "null", "perl", "ping", "plug",
		"pool", "post", "proc", "quad", "quiz", "rake", "rest", "ruby", "slug",
		"snap", "soap", "tail", "temp", "term", "unix", "uuid", "void", "wiki",
		"yoga", "zoom",
	}

	// 5字母单词
	fiveLetterWords := []string{
		"about", "above", "abuse", "actor", "acute", "admit", "adopt", "adult",
		"after", "again", "agent", "agree", "ahead", "alarm", "album", "alert",
		"alien", "align", "alike", "alive", "allow", "alone", "along", "alter",
		"amber", "amend", "among", "anger", "angle", "angry", "apart", "apple",
		"apply", "arena", "argue", "arise", "array", "arrow", "aside", "asset",
		"atlas", "audio", "audit", "avoid", "awake", "award", "aware", "badly",
		"baker", "bases", "basic", "beach", "began", "begin", "being", "below",
		"bench", "billy", "birth", "black", "blame", "blank", "blast", "blind",
		"block", "blood", "bloom", "board", "boost", "booth", "bound", "brain",
		"brand", "brass", "brave", "bread", "break", "breed", "brief", "bring",
		"broad", "broke", "brown", "brush", "build", "built", "buyer", "cable",
		"cache", "candy", "carry", "catch", "cause", "chain", "chair", "chaos",
		"charm", "chart", "chase", "cheap", "check", "chess", "chest", "child",
		"china", "chose", "civil", "claim", "class", "clean", "clear", "click",
		"climb", "clock", "close", "cloud", "coach", "coast", "could", "count",
		"court", "cover", "crack", "craft", "crash", "crazy", "cream", "crime",
		"cross", "crowd", "crown", "crude", "curve", "cycle", "daily", "dance",
		"dated", "dealt", "death", "debut", "delay", "depth", "doing", "doubt",
		"dozen", "draft", "drama", "drank", "dream", "dress", "drill", "drink",
		"drive", "drove", "dying", "eager", "early", "earth", "eight", "elite",
		"empty", "enemy", "enjoy", "enter", "entry", "equal", "error", "event",
		"every", "exact", "exist", "extra", "faith", "false", "fault", "fiber",
		"field", "fifth", "fifty", "fight", "final", "first", "fixed", "flash",
		"fleet", "floor", "fluid", "focus", "force", "forth", "forty", "forum",
		"found", "frame", "frank", "fraud", "fresh", "front", "fruit", "fully",
		"funny", "giant", "given", "glass", "globe", "glory", "grace", "grade",
		"grand", "grant", "grass", "grave", "great", "green", "gross", "group",
		"grown", "guard", "guess", "guest", "guide", "happy", "harry", "heart",
		"heavy", "hence", "henry", "horse", "hotel", "house", "human", "hurry",
		"image", "imply", "index", "inner", "input", "intro", "issue", "japan",
		"jimmy", "joint", "jones", "judge", "known", "label", "large", "laser",
		"later", "laugh", "layer", "learn", "lease", "least", "leave", "legal",
		"level", "lewis", "light", "limit", "links", "lived", "local", "logic",
		"loose", "lower", "lucky", "lunch", "lying", "magic", "major", "maker",
		"march", "maria", "match", "maybe", "mayor", "meant", "media", "metal",
		"might", "minor", "minus", "mixed", "model", "money", "month", "moral",
		"motor", "mount", "mouse", "mouth", "moved", "movie", "music", "needs",
		"never", "newly", "night", "noise", "north", "noted", "novel", "nurse",
		"occur", "ocean", "offer", "often", "order", "organ", "other", "ought",
		"owner", "paint", "panel", "panic", "paper", "party", "peace", "peter",
		"phase", "phone", "photo", "piano", "piece", "pilot", "pitch", "place",
		"plain", "plane", "plant", "plate", "point", "pound", "power", "press",
		"price", "pride", "prime", "print", "prior", "prize", "proof", "proud",
		"prove", "queen", "quick", "quiet", "quite", "radio", "raise", "range",
		"rapid", "ratio", "reach", "react", "ready", "realm", "rebel", "refer",
		"relax", "repay", "reply", "right", "rigid", "rival", "river", "robin",
		"roger", "roman", "rough", "round", "route", "royal", "rugby", "rural",
		"safer", "saint", "salad", "sales", "sarah", "sauce", "scale", "scare",
		"scene", "scope", "score", "sense", "serve", "seven", "shade", "shake",
		"shall", "shame", "shape", "share", "sharp", "sheep", "sheet", "shelf",
		"shell", "shift", "shine", "shirt", "shock", "shoot", "short", "shown",
		"sight", "silly", "simon", "since", "sixth", "sixty", "sized", "skill",
		"sleep", "slide", "small", "smart", "smile", "smith", "smoke", "snake",
		"snow", "solid", "solve", "sorry", "sound", "south", "space", "spare",
		"speak", "speed", "spend", "spent", "split", "spoke", "sport", "staff",
		"stage", "stake", "stand", "start", "state", "steal", "steam", "steel",
		"stick", "still", "stock", "stone", "stood", "store", "storm", "story",
		"strip", "stuck", "study", "stuff", "style", "sugar", "suite", "super",
		"sweet", "swift", "swing", "swiss", "sword", "table", "taken", "taste",
		"taxes", "teach", "terry", "thank", "theft", "their", "theme", "there",
		"these", "thick", "thing", "think", "third", "those", "three", "threw",
		"throw", "thumb", "tiger", "tight", "times", "title", "today", "token",
		"topic", "total", "touch", "tough", "tower", "track", "trade", "train",
		"treat", "trend", "trial", "tribe", "trick", "tried", "tries", "truly",
		"trunk", "trust", "truth", "twice", "twist", "tyler", "ultra", "uncle",
		"under", "undue", "union", "unity", "until", "upper", "upset", "urban",
		"urged", "usage", "users", "using", "usual", "valid", "value", "video",
		"virus", "visit", "vital", "vocal", "voice", "waste", "watch", "water",
		"wheel", "where", "which", "while", "white", "whole", "whose", "woman",
		"women", "world", "worry", "worse", "worst", "worth", "would", "write",
		"wrong", "wrote", "young", "yours", "youth", "admin", "adobe", "agent",
		"alert", "anime", "apple", "ascii", "atlas", "badge", "beach", "bench",
		"black", "blade", "blank", "blast", "blend", "blind", "block", "bloom",
		"board", "boost", "booth", "bound", "brain", "brand", "brave", "bread",
		"break", "brick", "brief", "bring", "broad", "brown", "brush", "build",
		"burst", "buyer", "cable", "cache", "candy", "carry", "catch", "chain",
		"chair", "chaos", "charm", "chart", "chase", "cheap", "check", "chess",
		"chest", "china", "chose", "civic", "claim", "class", "clean", "clear",
		"click", "climb", "clock", "close", "cloud", "clown", "coach", "coast",
		"could", "count", "court", "cover", "crack", "craft", "crash", "crazy",
		"cream", "crime", "crisp", "cross", "crowd", "crown", "crude", "curve",
		"cycle", "daily", "dance", "dated", "dealt", "death", "debug", "delay",
		"depth", "doing", "doubt", "dozen", "draft", "drama", "drank", "dream",
		"dress", "drill", "drink", "drive", "drove", "dying", "eager", "early",
		"earth", "eight", "elite", "empty", "enemy", "enjoy", "enter", "entry",
		"equal", "error", "event", "every", "exact", "exist", "extra", "faith",
		"false", "fault", "fiber", "field", "fifth", "fifty", "fight", "final",
		"first", "fixed", "flash", "fleet", "floor", "fluid", "focus", "force",
		"forth", "forty", "forum", "found", "frame", "frank", "fraud", "fresh",
		"front", "fruit", "fully", "funny", "giant", "given", "glass", "globe",
		"glory", "grace", "grade", "grand", "grant", "grass", "grave", "great",
		"green", "gross", "group", "grown", "guard", "guess", "guest", "guide",
		"happy", "harry", "heart", "heavy", "hence", "henry", "horse", "hotel",
		"house", "human", "hurry", "image", "imply", "index", "inner", "input",
		"intro", "issue", "japan", "jimmy", "joint", "jones", "judge", "known",
		"label", "large", "laser", "later", "laugh", "layer", "learn", "lease",
		"least", "leave", "legal", "level", "lewis", "light", "limit", "links",
		"lived", "local", "logic", "loose", "lower", "lucky", "lunch", "lying",
		"magic", "major", "maker", "march", "maria", "match", "maybe", "mayor",
		"meant", "media", "metal", "might", "minor", "minus", "mixed", "model",
		"money", "month", "moral", "motor", "mount", "mouse", "mouth", "moved",
		"movie", "music", "needs", "never", "newly", "night", "noise", "north",
		"noted", "novel", "nurse", "occur", "ocean", "offer", "often", "order",
		"organ", "other", "ought", "owner", "paint", "panel", "panic", "paper",
		"party", "peace", "peter", "phase", "phone", "photo", "piano", "piece",
		"pilot", "pitch", "place", "plain", "plane", "plant", "plate", "point",
		"pound", "power", "press", "price", "pride", "prime", "print", "prior",
		"prize", "proof", "proud", "prove", "queen", "quick", "quiet", "quite",
		"radio", "raise", "range", "rapid", "ratio", "reach", "react", "ready",
		"realm", "rebel", "refer", "relax", "repay", "reply", "right", "rigid",
		"rival", "river", "robin", "roger", "roman", "rough", "round", "route",
		"royal", "rugby", "rural", "safer", "saint", "salad", "sales", "sarah",
		"sauce", "scale", "scare", "scene", "scope", "score", "sense", "serve",
		"seven", "shade", "shake", "shall", "shame", "shape", "share", "sharp",
		"sheep", "sheet", "shelf", "shell", "shift", "shine", "shirt", "shock",
		"shoot", "short", "shown", "sight", "silly", "simon", "since", "sixth",
		"sixty", "sized", "skill", "sleep", "slide", "small", "smart", "smile",
		"smith", "smoke", "snake", "solid", "solve", "sorry", "sound", "south",
		"space", "spare", "speak", "speed", "spend", "spent", "split", "spoke",
		"sport", "staff", "stage", "stake", "stand", "start", "state", "steal",
		"steam", "steel", "stick", "still", "stock", "stone", "stood", "store",
		"storm", "story", "strip", "stuck", "study", "stuff", "style", "sugar",
		"suite", "super", "sweet", "swift", "swing", "swiss", "sword", "table",
		"taken", "taste", "taxes", "teach", "terry", "thank", "theft", "their",
		"theme", "there", "these", "thick", "thing", "think", "third", "those",
		"three", "threw", "throw", "thumb", "tiger", "tight", "times", "title",
		"today", "token", "topic", "total", "touch", "tough", "tower", "track",
		"trade", "train", "treat", "trend", "trial", "tribe", "trick", "tried",
		"tries", "truly", "trunk", "trust", "truth", "twice", "twist", "tyler",
		"ultra", "uncle", "under", "undue", "union", "unity", "until", "upper",
		"upset", "urban", "urged", "usage", "users", "using", "usual", "valid",
		"value", "video", "virus", "visit", "vital", "vocal", "voice", "waste",
		"watch", "water", "wheel", "where", "which", "while", "white", "whole",
		"whose", "woman", "women", "world", "worry", "worse", "worst", "worth",
		"would", "write", "wrong", "wrote", "young", "yours", "youth",
	}

	// 将所有单词组合
	allWords := [][]string{
		oneLetterWords,
		twoLetterWords,
		threeLetterWords,
		fourLetterWords,
		fiveLetterWords,
	}

	// 随机选择长度类别
	lengthIndex := rng.Intn(len(allWords))
	selectedWords := allWords[lengthIndex]

	// 从选中的长度类别中随机选择一个单词
	baseWord := selectedWords[rng.Intn(len(selectedWords))]
	baseWord = strings.ToLower(baseWord)

	// 只保留 a-z，丢弃词表中可能混入的非字母字符
	cleaned := make([]byte, 0, 5)
	for i := 0; i < len(baseWord); i++ {
		c := baseWord[i]
		if c >= 'a' && c <= 'z' {
			cleaned = append(cleaned, c)
		}
	}
	baseWord = string(cleaned)

	// 不足 5 位用小写字母填充（禁止数字，否则 HexReplacer/ValidateMagicName 会失败）
	for len(baseWord) < 5 {
		baseWord += string(letters[rng.Intn(len(letters))])
	}
	if len(baseWord) > 5 {
		baseWord = baseWord[:5]
	}

	return baseWord
}

// IsFridaNewName 检查是否为合法魔改名：恰好 5 个小写字母 a-z（与 core.ValidateMagicName 一致）
func IsFridaNewName(s string) bool {
	if len(s) != 5 {
		return false
	}
	for _, c := range s {
		if c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}