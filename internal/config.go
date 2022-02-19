package internal

//Configuration provides the different items we can use to
// configure how we connect to the database
type Configuration struct {
	Hostname  string `json:"hostname"`   //hostame to user to access the database
	Port      string `json:"port"`       //port to connect to
	Username  string `json:"username"`   //username to authenticate with
	Password  string `json:"password"`   //password to authenticate with
	Database  string `json:"database"`   //database to connect to
	ParseTime bool   `json:"parse_time"` //whether or not to parse time
}

//ConfigFromEnv can be used to generate a configuration pointer
// from a list of environments, it'll set the default configuraton
// as well
func ConfigFromEnv(envs map[string]string) *Configuration {
	c := &Configuration{
		Hostname:  "localhost",
		Port:      "3306",
		Username:  "root",
		Password:  "mysql",
		Database:  "bludgeon",
		ParseTime: false,
	}
	if hostname, ok := envs["HOSTNAME"]; ok {
		c.Hostname = hostname
	}
	if port, ok := envs["PORT"]; ok {
		c.Port = port
	}
	if username, ok := envs["USERNAME"]; ok {
		c.Username = username
	}
	if password, ok := envs["PASSWORD"]; ok {
		c.Password = password
	}
	if database, ok := envs["DATABASE"]; ok {
		c.Database = database
	}
	return c
}
