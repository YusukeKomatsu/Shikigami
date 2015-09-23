package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "net/url"
    "os"
    "os/user"
    "path/filepath"
    "strings"

    "golang.org/x/net/context"
    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
    gmail "google.golang.org/api/gmail/v1"

    "github.com/codegangsta/cli"
    "github.com/davecgh/go-spew/spew"
)

var me = "me"

func main() {
    app := cli.NewApp()
    app.Name = "proto"
    app.Usage = "test"
    app.Version = "0.0.1"
    app.Author = "Yusuke Komatsu"
    app.Commands = []cli.Command{
        {
            Name:        "cleanUp",
            Usage:       "---",
            Description: "",
            Action:      cleanUp,
        },
        {
            Name:        "deleteAwaitingResponse",
            Usage:       "---",
            Description: "",
            Action:      deleteAwaitingResponse,
        },
        {
            Name:        "getNoLabelSender",
            Usage:       "---",
            Description: "",
            Action:      getNoLabelSender,
        },
        {
            Name:        "labelMessagesWithoutResponse",
            Usage:       "---",
            Description: "",
            Action:      labelMessagesWithoutResponse,
        },
        {
            Name:        "setParentLabel",
            Usage:       "---",
            Description: "",
            Action:      setParentLabel,
        },
    }

    app.Run(os.Args)
}

func cleanUp(c *cli.Context) {
    srv, err := getNewService()
    if err != nil {
        log.Fatalf("Unable to retrieve gmail Client %v", err)
    }

    req, err := srv.Users.Threads.List(me).Q("label:3day older_than:3d").Do()
    if err != nil {
        log.Fatalf("Unable to retrieve threads. %v", err)
    }

    if (len(req.Threads) > 0) {
        for _, th := range req.Threads {
            _, err := srv.Users.Threads.Trash(me, th.Id).Do()
            if err != nil {
                log.Fatalf("Unable to trash thread. ID:%v, %v", th.Id, err)
            }
        }
    }
}

func labelMessagesWithoutResponse(c *cli.Context) {
    srv, err := getNewService()
    if err != nil {
        log.Fatalf("Unable to retrieve gmail Client %v", err)
    }

    req, err := srv.Users.Threads.List(me).Q("in:Sent -label:AwaitingResponse newer_than:7d").Do()
    if err != nil {
        log.Fatalf("Unable to retrieve threads. %v", err)
    }
    if len(req.Threads) > 0 {
        label_ar, err := getLabelByName("AwaitingResponse")
        if err != nil {
            log.Fatalf("Unable to retrieve AwaitingResponse label. %v", err)
        }
        label_fe, err := getLabelByName("FinishedExchange")
        if err != nil {
            log.Fatalf("Unable to retrieve FinishedExchange label. %v", err)
        }
        for _, th := range req.Threads {
            mod := &gmail.ModifyThreadRequest{}
            lbId := label_ar.Id
            if len(th.Messages) > 1 {
                lbId = label_fe.Id
            }
            mod.AddLabelIds = append(mod.AddLabelIds, lbId)
            _, err := srv.Users.Threads.Modify(me, th.Id, mod).Do()
            if err != nil {
                log.Fatalf("Unable to set label. %v", err)
            }
        }
    }
}

func deleteAwaitingResponse(c *cli.Context) {
    srv, err := getNewService()
    if err != nil {
        log.Fatalf("Unable to retrieve gmail Client %v", err)
    }

    req, err := srv.Users.Threads.List(me).Q("label:AwaitingResponse older_than:7d newer_than:14d").Do()
    if err != nil {
        log.Fatalf("Unable to retrieve threads. %v", err)
    }
    if len(req.Threads) > 0 {
        removelabel, err := getLabelByName("AwaitingResponse")
        if err != nil {
            log.Fatalf("Unable to retrieve AwaitingResponse label. %v", err)
        }
        addlabel, err := getLabelByName("FinishedExchange")
        if err != nil {
            log.Fatalf("Unable to retrieve FinishedExchange label. %v", err)
        }
        for _, th := range req.Threads {
            mod := &gmail.ModifyThreadRequest{}
            mod.RemoveLabelIds = append(mod.RemoveLabelIds, removelabel.Id)
            if len(th.Messages) > 1 {
                mod.AddLabelIds = append(mod.AddLabelIds, addlabel.Id)
            }
            _, err := srv.Users.Threads.Modify(me, th.Id, mod).Do()
            if err != nil {
                log.Fatalf("Unable to remove label. %v", err)
            }
        }
    }
}

func setParentLabel(c *cli.Context) {
    srv, err := getNewService()
    if err != nil {
        log.Fatalf("Unable to retrieve gmail Client %v", err)
    }

    req, err := srv.Users.Labels.List(me).Do()
    if err != nil {
        log.Fatalf("Unable to retrieve labels. %v", err)
    }

    if (len(req.Labels) > 0) {
        parents := make(map[string]string)
        for _, lb := range req.Labels {
            if (strings.Contains(lb.Name, "/") == false) {
                parents[lb.Name] = lb.Id
            }
        }

        for _, l := range req.Labels {
            lbName := l.Name
            if (strings.Contains(lbName, "/")) {
                splitedLabelNames := strings.Split(lbName, "/")
                parent := splitedLabelNames[0]
                query := fmt.Sprintf("label:%s -label:%s", lbName, parent)
                r, err := srv.Users.Threads.List(me).Q(query).Do()
                if err != nil {
                    log.Fatalf("Unable to retrieve threads. %v", err)
                }
                if parentId, ok := parents[parent]; ok {
                    mod := &gmail.ModifyThreadRequest{}
                    mod.AddLabelIds = append(mod.AddLabelIds, parentId)
                    for _, th := range r.Threads {
                        _, err := srv.Users.Threads.Modify(me, th.Id, mod).Do()
                        if err != nil {
                            log.Fatalf("Unable to set label. %v", err)
                        }
                    }                    
                }
            }
        }
    }
}

func getNoLabelSender(c *cli.Context) {
    srv, err := getNewService()
    if err != nil {
        log.Fatalf("Unable to retrieve gmail Client %v", err)
    }

    req, err := srv.Users.Threads.List(me).Q("has:nouserlabels newer_than:1d").Do()
    if err != nil {
        log.Fatalf("Unable to retrieve threads. %v", err)
    }
    if len(req.Threads) > 0 {
        sender := []string{}
        for _, th := range req.Threads {
            t, err := srv.Users.Threads.Get(me, th.Id).Fields("messages/payload").Do()
            if err != nil {
                log.Fatalf("Unable to retrieve a thread information. %v", err)
            }
            for _, msg := range t.Messages {
                headers := msg.Payload.Headers
                for _, header := range headers {
                    if header.Name == "From" {
                        sender = append(sender, header.Value)
                    }
                }
            }
        }
        spew.Dump(sender)
    }
}

func getLabelByName(labelName string) (*gmail.Label, error) {
    srv, err := getNewService()
    if err != nil {
        return nil, err
    }

    req, err := srv.Users.Labels.List(me).Do()
    if err != nil {
        return nil, err
    }

    if (len(req.Labels) > 0) {
        for _, l := range req.Labels {
            if (l.Name == labelName) {
                return l, nil
            }
        }
    }
    return nil, nil
}

func getNewService() (*gmail.Service, error) {
    b, err := ioutil.ReadFile("client_secret.json")
    if err != nil {
        log.Fatalf("Unable to read client secret file: %v", err)
    }

    config, err := google.ConfigFromJSON(b, gmail.MailGoogleComScope)
    if err != nil {
        log.Fatalf("Unable to parse client secret file to config: %v", err)
    }
    client := getClient(context.Background(), config)

    srv, err := gmail.New(client)
    return srv, err
}

// getClient uses a Context and Config to retrieve a Token
// then generate a Client. It returns the generated Client.
func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
  cacheFile, err := tokenCacheFile()
  if err != nil {
    log.Fatalf("Unable to get path to cached credential file. %v", err)
  }
  tok, err := tokenFromFile(cacheFile)
  if err != nil {
    tok = getTokenFromWeb(config)
    saveToken(cacheFile, tok)
  }
  return config.Client(ctx, tok)
}

// getTokenFromWeb uses Config to request a Token.
// It returns the retrieved Token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
  authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
  fmt.Printf("Go to the following link in your browser then type the "+
    "authorization code: \n%v\n", authURL)

  var code string
  if _, err := fmt.Scan(&code); err != nil {
    log.Fatalf("Unable to read authorization code %v", err)
  }

  tok, err := config.Exchange(oauth2.NoContext, code)
  if err != nil {
    log.Fatalf("Unable to retrieve token from web %v", err)
  }
  return tok
}

// tokenCacheFile generates credential file path/filename.
// It returns the generated credential path/filename.
func tokenCacheFile() (string, error) {
  usr, err := user.Current()
  if err != nil {
    return "", err
  }
  tokenCacheDir := filepath.Join(usr.HomeDir, ".credentials")
  os.MkdirAll(tokenCacheDir, 0700)
  return filepath.Join(tokenCacheDir,
    url.QueryEscape("token_cache.json")), err
}

// tokenFromFile retrieves a Token from a given file path.
// It returns the retrieved Token and any read error encountered.
func tokenFromFile(file string) (*oauth2.Token, error) {
  f, err := os.Open(file)
  if err != nil {
    return nil, err
  }
  t := &oauth2.Token{}
  err = json.NewDecoder(f).Decode(t)
  defer f.Close()
  return t, err
}

// saveToken uses a file path to create a file and store the
// token in it.
func saveToken(file string, token *oauth2.Token) {
  fmt.Printf("Saving credential file to: %s\n", file)
  f, err := os.Create(file)
  if err != nil {
    log.Fatalf("Unable to cache oauth token: %v", err)
  }
  defer f.Close()
  json.NewEncoder(f).Encode(token)
}