package main

import (
	"bufio"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"internal/common"
	"log"
	"net"
	"os"
	"strconv"
)

// Message structure that represents messages sent between clients.
type Message struct {
	To      string
	From    string
	Content string
	Error   bool
}

// Connection structure that represents a 1 to 1 connection between server and client.
type Connection struct {
	username string
	conn     net.Conn
}

/*
Accept a connection request and return a struct identifying the connecting user.
When a new user connects, they send their username prefixed by a header indicating the length
of the username. The username is rejected if the length is too high because we don't
want malicious clients causing the server to allocate extra memory.
*/
func receive_connection(ln net.Listener) (Connection, error) {
	conn, err := ln.Accept()

	if err != nil {
		return Connection{}, err
	}

	// Size of username in bytes
	var username_size uint8
	err = binary.Read(conn, binary.BigEndian, &username_size)

	if err != nil {
		conn.Close()
		return Connection{}, err
	}

	// Reject long usernames. Errors are not sent to the client because we assume the client is malicious. Our
	// client does not allow you to send a long username
	if username_size > uint8(common.MAX_USERNAME_LENGTH) {
		conn.Close()
		return Connection{}, fmt.Errorf("username was too long: max %d characters", common.MAX_USERNAME_LENGTH)
	}

	raw_data := make([]byte, username_size)
	_, err = conn.Read(raw_data)

	if err != nil {
		conn.Close()
		return Connection{}, err
	}

	username := string(raw_data)

	fmt.Fprintf(common.ColorOutput, "User %s connected\n", common.NameColor(username))
	return Connection{username, conn}, nil
}

/*
* A function that runs continously, decodes
* messages received from source clients and validates
* the username and message content. Upon successful
* validation, puts the message in the server's message queue.
 */
func receive_messages(connections map[string]Connection, conn Connection, mq chan<- Message) {
	for {

		dec := gob.NewDecoder(conn.conn)
		var msg Message
		err := dec.Decode(&msg)
		if err != nil {
			break
		}
		to_size := uint16(len(msg.To))
		msg_size := uint16(len(msg.Content))

		if to_size > uint16(common.MAX_USERNAME_LENGTH) {
			fmt.Fprintf(
				common.ColorOutput,
				"User %s tried to send a message to a username of length %d. Maximum length is %d.\n",
				common.NameColor(conn.username),
				to_size,
				common.MAX_USERNAME_LENGTH)
			break
		}

		if msg_size > uint16(common.MAX_MESSAGE_LENGTH) {
			fmt.Fprintf(
				common.ColorOutput,
				"User %s tried to send a message of length %d. Maximum length is %d.\n",
				common.NameColor(conn.username),
				msg_size,
				common.MAX_MESSAGE_LENGTH)
			break
		}
		msg.From = conn.username
		mq <- msg
		fmt.Fprintf(common.ColorOutput, "%s to %s: %s\n", common.NameColor(conn.username), common.NameColor(msg.To), common.MessageColor(msg.Content))
	}

	conn.conn.Close()
	delete(connections, conn.username)
	fmt.Fprintf(common.ColorOutput, "User %s disconnected or kicked\n", common.NameColor(conn.username))

}

// A utility function that sends an error message
// from server to client.
func send_error(conn net.Conn, error_msg string) {
	msg := Message{"", "", error_msg, true}
	enc := gob.NewEncoder(conn)
	err := enc.Encode(msg)
	if err != nil {
		log.Fatal(err)
	}
}

/*
* A function that continually processes the message queue.
* Pulls messages from the queue and checks their source and destination
* fields. If both source and destination clients have disconnected the
function drops the message. If the destination client is not connected
the function sends an error message to the source client and drops the message.
If all checks pass, the function sends the message to the destination client.
*/
func process_message_queue(connections map[string]Connection, mq <-chan Message) {
	for {
		msg, ok := <-mq

		if !ok {
			return
		}

		to, ok := connections[msg.To]

		if !ok {
			fmt.Fprintf(common.ColorOutput, "User %s does not exist\n", common.NameColor(msg.To))

			from, ok := connections[msg.From]
			// Obscure edge case in which a sender disconnects immediately after sending a message, but before the message
			// is delivered. `connections` is shared among threads so another thread could delete a sender
			// before this thread has a chance to process the message.dec
			if !ok {
				fmt.Fprintf(common.ColorOutput, "Sender %s does not exist either. Dropping message\n", common.NameColor(msg.From))
				continue
			}
			send_error(from.conn, fmt.Sprintf("'%s' is not connected\n", msg.To))
			continue
		}
		enc := gob.NewEncoder(to.conn)
		err := enc.Encode(msg)
		if err != nil {
			log.Fatal(err)
		}
	}
}

/*
* A function that listens for incoming connections on the server.
* The function checks the username of the source client initiating
* the connection to make sure its available. If available,
* the server saves the client's connection and starts
a new goroutine to process the source client's messages.
*/
func listen_for_connections(ln net.Listener, connections map[string]Connection, mq chan<- Message) {
	for {
		conn, err := receive_connection(ln)

		if err != nil {
			fmt.Println(err)
			continue
		}

		_, ok := connections[conn.username]

		if ok {
			send_error(conn.conn, "Username is taken\n")
			conn.conn.Close()
			continue
		}

		connections[conn.username] = conn
		go receive_messages(connections, conn, mq)
	}
}

/*
* Main thread, initializes the message queue
* as well as the map from client usernames to messages.
* The function starts two goroutines one to listen
* for connections and another to process the message queue
* the function then waits for an 'exit' command from the user
 */
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Need to supply listening port for incoming connections")
		return
	}

	listen_port, err := strconv.Atoi(os.Args[1])

	if err != nil {
		panic(err)
	}

	connections := make(map[string]Connection)
	message_queue := make(chan Message, 256)
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", listen_port))

	if err != nil {
		panic(err)
	}

	go process_message_queue(connections, message_queue)
	fmt.Printf("Listening for connections\n")

	go listen_for_connections(ln, connections, message_queue)

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		if scanner.Text() == "exit" {
			os.Exit(0)
		} else {
			fmt.Println("Unrecognized command. Type 'exit' to quit")
		}
	}
}
