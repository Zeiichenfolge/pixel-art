package main

import (
	context2 "context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/ravener/discord-oauth2"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strconv"
)

var Engine *gin.Engine
var Conf Config
var Pixels map[string]string
var DiscordConfig *oauth2.Config
var TokenCache = make(map[string]string)
var clients = make(map[*websocket.Conn]bool)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func main() {
	loadConfig()
	Engine = gin.Default()
	gin.SetMode(gin.ReleaseMode)
	Pixels = make(map[string]string)
	Engine.LoadHTMLGlob("./templates/*")
	Engine.SetTrustedProxies(nil)
	routes()
	DiscordConfig = &oauth2.Config{
		Endpoint:     discord.Endpoint,
		Scopes:       []string{discord.ScopeIdentify},
		RedirectURL:  "http://zeichenfolge.xyz:5050/login/callback",
		ClientID:     Conf.DiscordClientId,
		ClientSecret: Conf.DiscordClientSecret,
	}
	err := Engine.Run(":5050")
	if err != nil {
		panic("Engine failed to Start")
		return
	}
}

func routes() {
	// Index Call
	Engine.GET("/", func(context *gin.Context) {
		LoggedIn, _ := readIDCookie(context)
		context.HTML(http.StatusOK, "index.html", gin.H{
			"LoggedIn": LoggedIn,
		})
	})
	// Data Call
	Engine.GET("/pixels", func(context *gin.Context) {
		context.JSON(http.StatusOK, Pixels)
	})
	Engine.GET("/socket", func(context *gin.Context) {
		ws, err := upgrader.Upgrade(context.Writer, context.Request, nil)
		if err != nil {
			context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		clients[ws] = true
		defer ws.Close()
	})
	// Pixel set POST Call, used for pixel update
	Engine.POST("/updatePixel", func(context *gin.Context) {
		var data PixelData
		err := context.ShouldBindJSON(&data)
		if err != nil {
			return
		}
		Pixels[data.ID] = data.Color
		for client := range clients {
			err := client.WriteJSON(data)
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}
	})
	// Login Call for Login start
	Engine.GET("/login", func(context *gin.Context) {
		success, _ := readIDCookie(context)
		if success {
			context.Redirect(http.StatusTemporaryRedirect, "/")
			return
		}
		context.Redirect(http.StatusTemporaryRedirect, DiscordConfig.AuthCodeURL(Conf.DiscordClientSecretState))
	})
	// Login Callback for Login finish
	Engine.GET("/login/callback", func(context *gin.Context) {
		if context.Request.FormValue("state") != Conf.DiscordClientSecretState {
			context.Writer.WriteHeader(http.StatusBadRequest)
			context.Writer.Write([]byte("State does not match."))
			return
		}
		token, err := DiscordConfig.Exchange(context2.Background(), context.Request.FormValue("code"))

		if err != nil {
			fmt.Println("exchange error:", err.Error())
			panic(err)
			return
		}

		client := DiscordConfig.Client(context2.Background(), token)
		res, err := client.Get("https://discord.com/api/users/@me")
		if err != nil || res.StatusCode != 200 {
			context.Writer.WriteHeader(http.StatusInternalServerError)
			if err != nil {
				context.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
			} else {
				context.JSON(http.StatusInternalServerError, gin.H{
					"status": res.Status,
				})
			}
			return
		}
		defer res.Body.Close()
		if err != nil {
			context.Writer.WriteHeader(http.StatusInternalServerError)
			context.Writer.Write([]byte(err.Error()))
			return
		}
		discorduser := DiscordUser{}
		err = json.NewDecoder(res.Body).Decode(&discorduser)
		if err != nil {
			fmt.Println(err.Error())
		}
		atoi, _ := strconv.Atoi(discorduser.ID)
		Code := randomString(int64(atoi))
		context.SetCookie("temp_token", Code, 60*60*24*2, "/", "zeichenfolge.xyz", false, false)
		context.SetCookie("temp_id", discorduser.ID, 60*60*24*2, "/", "zeichenfolge.xyz", false, false)
		TokenCache[Code] = discorduser.ID
		context.Redirect(http.StatusTemporaryRedirect, "/")
	})

	// Logout Call
	Engine.POST("/logout", func(context *gin.Context) {
		cookie, _ := context.Cookie("temp_token")
		delete(TokenCache, cookie)
		context.SetCookie("temp_token", "1", 1, "/", "zeichenfolge.xyz", false, false)
		context.SetCookie("temp_id", "1", 1, "/", "zeichenfolge.xyz", false, false)
		context.Redirect(http.StatusTemporaryRedirect, "/")
	})
}

func readIDCookie(context *gin.Context) (bool, string) {
	tempToken, err1 := context.Cookie("temp_token")
	if err1 != nil {
		return false, err1.Error()
	}
	tempId, err2 := context.Cookie("temp_id")
	if err2 != nil {
		return false, err2.Error()
	}
	if TokenCache[tempToken] == tempId {
		return true, tempId
	}
	return false, "failed"
}

func randomString(id int64) string {
	rand.Seed(id)
	letters := "AaBbCcDdEeFfGgHhIiJjKkLlMmNnOoPpQqRrSsTtUuVvWwXxYyZz0123456789.-_,;:#*!$%&/()}][{"
	b := make([]byte, 32)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	code := string(b)
	runes := []rune(code)
	rand.Shuffle(len(code), func(i, j int) {
		runes[i], runes[j] = runes[j], runes[i]
	})
	return string(runes)
}

func loadConfig() {
	raw, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Println("Error occurred while reading config")
		return
	}
	err = json.Unmarshal(raw, &Conf)
	fmt.Println(Conf)
	if err != nil {
		return
	}
}

type PixelData struct {
	ID    string `json:"id"`
	Color string `json:"color"`
}

type Config struct {
	Port                     int64  `json:"port"`
	DiscordClientId          string `json:"discordclientid"`
	DiscordClientSecret      string `json:"discordclientsecret"`
	DiscordClientSecretState string `json:"discordclientsecretstate"`
}

type DiscordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}
