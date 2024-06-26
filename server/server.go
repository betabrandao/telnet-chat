package server

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/betabrandao/telnet-chat/config"
	"github.com/betabrandao/telnet-chat/connection"
	"github.com/betabrandao/telnet-chat/room"
)

type Server struct {
	Running  bool
	Listener net.Listener
	Rooms    []*room.Room
	LogFile  *os.File
}

var helpMessage string = `
Alguns comandos disponiveis no char:
/ajuda: exibe esta ajuda
/nome: altera seu nome
/xau: sai da sala atual. 
/sair: sai do telnet-chat.
`

// Return a stringified list of rooms
func (s *Server) ListRooms() string {
	str := "Salas disponiveis:\n"
	for i, room := range s.Rooms {
		str += fmt.Sprintf("\t%d: %s\n", i, room.Name)
	}
	return str
}

// Allow the user to select a room, sets the connection's index to the index of
// the room in the server struct's list of rooms
func (s *Server) SelectRoom(c *connection.Connection) error {
	roomList := s.ListRooms()
  if err := c.SendMessage("Selecione a sala que deseja entrar digitando o numero da sala:\n"); err != nil {
		return fmt.Errorf("Failed to send message to %q (%s): %s",
			c.UserName, c.Conn.RemoteAddr(), err.Error())
	}
	room, err := c.SendWithResponse(roomList)
	if err != nil {
		return fmt.Errorf("Failed to send message to %q (%s): %s\n",
			c.UserName, c.Conn.RemoteAddr(), err.Error())
	}

	if room == "" {
		c.SendError("Necessario uma sala para entrar...")
		return fmt.Errorf("User %q (%s) failed to choose room\n", c.UserName, c.Conn.RemoteAddr())
	}

	roomIndex, err := strconv.Atoi(room)
	if err != nil {
		return fmt.Errorf("Error choosing room for user %s: %s\n", c.String(), err.Error())
	} else if roomIndex > len(s.Rooms) || roomIndex < 0 {
		return fmt.Errorf("User %s selected invalid room\n", c.String())
	}

	s.Rooms[roomIndex].AddUser(c)
	c.Room = roomIndex

	return nil
}

// Handle various user commands. Only available when in a room
func (s *Server) HandleCommands(message string, c *connection.Connection) bool {
	switch message {
	case "/ajuda":
		if err := c.SendMessage(helpMessage); err != nil {
			log.Println(err)
			return true
		}
	case "/nome":
		newName, err := c.SendWithResponse("Novo nome: ")
		if err != nil {
			log.Println(err)
			return true
		}
		log.Printf("User %s changed name to %s\n", c.String(), newName)
		s.Rooms[c.Room].WriteMessage(fmt.Sprintf("Usuario %s mudou o nome para %s\n", c.UserName, newName))
		c.UserName = newName

		return true
	case "/xau":
		room := s.Rooms[c.Room]
		room.RemoveUser(c)
		s.SelectRoom(c)
		return true
	case "/sair":
		c.Close()
		return true
	}
	return false
}

// Handle user messages to a room as well as commands. Exits when the user disconnects
func (s *Server) HandleMessages(c *connection.Connection) {
	for c.Open == true {
		text, err := c.SendWithResponse(">> ")
		if err != nil {
			log.Printf("Failed to read message from %s: %s", c.String(), err.Error())
			return
		}

		if s.HandleCommands(text, c) == true {
			continue
		}

		message := fmt.Sprintf("<%s> (%s): %s\n", time.Now().Format(time.Kitchen), c.UserName, text)
		room := s.Rooms[c.Room]
		room.WriteMessage(message)

		logStr := fmt.Sprintf("%s: %s", room.Name, message)
		_, err = s.LogFile.WriteString(logStr)
		if err != nil {
			log.Printf("Failed to log message from user %s: %s", c.String(), err.Error())
		}

		log.Printf("User %s sent message %q to room %q\n", c.String(), text, room.Name)
	}
}

// Initialize the connection object and start a go routine to handle messaging with the client
func (s *Server) HandleConnection(c *connection.Connection) {
	username, err := c.SendWithResponse("Nome desejado: ")
	if err != nil || username == "" {
		c.Close()
		log.Println("User failed to enter username")
		return
	}

	c.UserName = username
	log.Printf("User %s connected\n", c.String())

	if err := s.SelectRoom(c); err != nil {
		log.Println(err)
		c.Close()
		return
	}
	go s.HandleMessages(c)
}

// Start the server's room go-routines, start the tcp listener and handle incoming connections
func (s *Server) Serve() {

	for _, room := range s.Rooms {
		log.Printf("Starting room %q...\n", room.Name)
		go room.Run()
	}

	for s.Running {
		conn, err := s.Listener.Accept()
		if err != nil {
			log.Println("Error accepting connection:", err)
			continue
		}

		c := connection.NewConnection(conn)
		go s.HandleConnection(c)
	}
}

// Initialize the rooms in a server
func (s *Server) InitializeRooms() {
	for _, roomName := range config.Config.Rooms {
		log.Printf("Initializing room %q\n", roomName)
		s.Rooms = append(s.Rooms, &room.Room{
			Name:        roomName,
			Connections: make(map[string]*connection.Connection, 0),
			WriteChan:   make(chan string),
		})
	}
}

// Initiaize a new server with setttings read from the configuration file
func NewServer() (*Server, error) {

	bindAddr := config.Config.BindAddr + ":" + config.Config.BindPort

	log.Println("Starting listener on", bindAddr)
	listener, err := net.Listen("tcp4", bindAddr)
	if err != nil {
		return nil, err
	}

	log.Printf("Opening message log file %q\n", config.Config.LogFile)
	f, err := os.OpenFile(config.Config.LogFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return nil, err
	}

	s := &Server{
		Running:  true,
		Listener: listener,
		Rooms:    make([]*room.Room, 0),
		LogFile:  f,
	}

	s.InitializeRooms()

	return s, nil
}
