/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	// "github.com/charmbracelet/bubbles/list"
	// "github.com/charmbracelet/bubbles/table"
	// "github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	
)

//------Styles---------
var (
	appStyle = lipgloss.NewStyle().Padding(1,2)
	titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5")).Background(lipgloss.Color("#00BFFF")).Padding(0,1)
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	successStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#32CD32"))
	errorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF0000"))
	containerStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63"))
)

//------TUI States---------
// type appState int
// const(
// 	stateURLInput appState = iota
// 	stateFormatSelection
// 	stateDownloading
// 	stateFinished
// )

// //-------List Item for FORMAT Selection---------
// type formatItem struct{
// 	code string 
// 	extension string
// 	resolution string
// 	description string 
// }

// func (i formatItem) Title() string {return fmt.Sprintf("%s (%s)", i.resolution, i.extension)}
// func (i formatItem) Description() string	{return i.description}
// func (i formatItem) FilterValue() string {return i.code + " " + i.resolution + " " + i.extension}



//Variable to store the values of our flags
var(
	outputPath string
	quality string 
	audioOnly bool
	startTime string 
	endTime string 
)

//Communication messages b/w the Downloading Task and the User-Interface
type downloadFinishedMsg struct{
	err error
}
// type formatFetchedMsg struct{items []list.Item}
// type formatSelectedMsg struct{}
type outputLineMsg struct{line string}
type readOutputMsg struct{}
type downloadProgressMsg struct{output string}
type progressTickMsg struct{}


//========Bubble-Tea Model struct=============
type model struct{
	spinner spinner.Model
	progressBar progress.Model
	videoURL string
	outputPath string
	quality string
	audioOnly bool 
	startTime string 
	endTime string
	
	cmd *exec.Cmd
	scanner *bufio.Scanner
	done bool
	err error
	progress []string
}

//----------InitialModel creates the starting state for our TUI--------------
func initialModel(url, path, quality string, audio bool, start, end string) model{
	s := spinner.New() //Create a new spinner.model instance
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF"))
	return model{
		spinner: s,
		progressBar: progress.New(progress.WithDefaultGradient()),
		videoURL: url,
		outputPath: path,
		quality: quality,
		audioOnly: audio,
		startTime: start,
		endTime: end,
		progress: make([]string, 0),
	}
}

//---------------Init Command---------------------
func (m *model) Init() tea.Cmd{
	//Start the Spinner ticking and run the download command
	var args []string

	if m.audioOnly{
		//Audio-Only command
		args = []string{"-x", "--audio-format", "mp3"}
	}else{
		//Video Command
		args = []string{"--progress", "-f", "bv[ext=mp4]+ba[ext=m4a]/b[ext=mp4]"}
		if m.quality != ""{
			args[2] = fmt.Sprintf("bv[height<=?%s][ext=mp4]+ba[ext=m4a]/b[ext=mp4]", m.quality)
		}
		args = append(args,"--merge-output-format", "mp4")
	}
	//Add Output-Path if Output flag is checked
	if m.outputPath != ""{
		args = append(args, "-o", m.outputPath)
	}

	//Add StartTime,EndTime if start,end flag are checked
	if m.startTime != "" || m.endTime != ""{
		timeRange := fmt.Sprintf("*%s-%s", m.startTime, m.endTime)
		args = append(args, "--download-sections", timeRange)
	}
	args = append(args, m.videoURL)


	m.cmd = exec.Command("yt-dlp", args...)
	//Get a PIPE 
	stdout, err := m.cmd.StdoutPipe()
	if err != nil{
		m.err = err
		return tea.Quit
	}

	m.cmd.Stderr = m.cmd.Stdout
	m.scanner = bufio.NewScanner(stdout)

	//Start the Command and the Spinner
	err = m.cmd.Start()
	if err != nil{
		m.err = err
		return tea.Quit
	}

	//Start reading the Output and ticking the spinner
	return tea.Batch(m.spinner.Tick, readOutput(m.scanner), progressTick())

	//This code was used when we just had a static output by the TUI
	// return tea.Batch(m.spinner.Tick, runDownload(m.videoURL, m.outputPath, m.quality, m.audioOnly, m.startTime, m.endTime))
}

//------------Update Command{Handles the messages and updates the model}----------------
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd){
	switch msg := msg.(type){

		//Case- when key is pressed
		case tea.KeyMsg:
			switch msg.String(){
				case "ctrl+c", "q":
					m.done = true
					if m.cmd != nil && m.cmd.Process != nil{
						m.cmd.Process.Kill()
					}
					return m, tea.Quit
			}

		//Case- when output line message
		case outputLineMsg:
			if m.done{
				return m,nil
			}

			m.progress = append(m.progress, msg.line)

			r := regexp.MustCompile(`\s+(\d+(?:\.\d+)?)%`)
			matches := r.FindStringSubmatch(msg.line)

			if len(matches) > 1{
				percent , err := strconv.ParseFloat(matches[1], 64)
				// if err == nil{
				// 	//It means we have a new progress percentage, so we send a command to update the progress bar.
				// 	return m, tea.Batch(m.progressBar.SetPercent(percent/100.0), readOutput(m.scanner))
				// }

				if err == nil{
					return m, tea.Batch(
						m.progressBar.SetPercent(percent / 100.0),
						func() tea.Msg{
							if m.done{
								return nil 
							}
							return readOutput(m.scanner)()
						},
					)
				}
			}

			if !m.done{
				return m, readOutput(m.scanner)
			}
			return m, nil 
			// return m, readOutput(m.scanner)
		//Case- When download is complete
		case downloadFinishedMsg:
			m.done = true
			// m.err = m.cmd.Wait()
			m.err = msg.err
			// m.output = msg.output
			// return m,tea.Batch(
			// 	 m.progressBar.SetPercent(1.0),
			// 	 tea.Quit)
			return m, m.progressBar.SetPercent(1.0)
		
		case spinner.TickMsg:
			if m.done{
				return m,nil
			}
			var cmd tea.Cmd
			m.spinner , cmd = m.spinner.Update(msg)
			return m, cmd

		case progress.FrameMsg:
			progressModel, cmd := m.progressBar.Update(msg)
			m.progressBar = progressModel.(progress.Model)
			if m.done && m.progressBar.Percent() == 1.0{
				return m, tea.Quit
			}
			return m,cmd
	}

	return m, nil
		
}

//--------------View command{Renders the UI}--------------
func (m *model) View() string{
	var b strings.Builder
	if m.done {
		// Final View
		b.WriteString(titleStyle.Render(":) Download Complete"))
		b.WriteString("\n\n")
		b.WriteString(m.progressBar.View()) // Render the 100% progress bar
		b.WriteString("\n\n")
		if m.err != nil {
			b.WriteString(errorStyle.Render(fmt.Sprintf("❌ Error: %v", m.err)))
		} else {
			b.WriteString(successStyle.Render("✅ Success! Your file is ready."))
		}
	} else {
		// Active View
		b.WriteString(titleStyle.Render("Downloading Video"))
		b.WriteString("\n\n")
		b.WriteString(m.progressBar.View())
		b.WriteString("\n\n")
		b.WriteString(m.spinner.View())
		b.WriteString(" Running...")
		b.WriteString(helpStyle.Render("\n(Press 'q' or 'ctrl+c' to quit)"))
	}

	return appStyle.Render(b.String())
}

//=======================Helper Function========================
//It returns a tea.Cmd which is a function that Bubble Tea will run for us

//-----------Function to tell the TUI to read the next line of output----------------
func readOutput(s *bufio.Scanner) tea.Cmd{
	return func() tea.Msg{
		if s.Scan(){
			return outputLineMsg{line : s.Text()}
		}
		return downloadFinishedMsg{err: s.Err()}
	}
}

//--------------------Function to send a tick message for the progress bar animation---------------
func progressTick() tea.Cmd{
	return tea.Tick(time.Second/4, func(t time.Time) tea.Msg{
		return progressTickMsg{}
	})
}


// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "Command-Line Tool for downloading from Youtube",
	Short: "A simple CLI to download from Youtube using yt-dlp",
	Long: `This is a custom Go CLI tool that downloads the youtube audio/video when provided with a URL.`,
	Args: cobra.ExactArgs(1),

	//This is the main function that will run when our command is called
	Run: func(cmd *cobra.Command, args []string){
		videoURL := args[0]
		
		//Simple Flag Validation
		if audioOnly && quality != ""{
			fmt.Println("Error: Please Don't use --quality flag with --audio-only flag")
			return 
		}

		//Ensure yt-dlp is installed
		_, err := exec.LookPath("yt-dlp")
		if err != nil {
			fmt.Println(":( yt-dlp not found in PATH. Please install it to continue.")
			return
		}

		//Create the Bubble-Tea model
		m := initialModel(videoURL, outputPath, quality, audioOnly, startTime, endTime)
		p := tea.NewProgram(&m)

		if _, err := p.Run(); err!=nil{
			fmt.Printf(":( Error occured when running the program: %v\n", err)
			os.Exit(1)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cli-youtube-downloader.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Specify output filename and path:")
	rootCmd.Flags().StringVarP(&quality, "quality", "q", "", "Specify video quality(e.g 720, 1080)")
	rootCmd.Flags().BoolVar(&audioOnly, "audio-only", false, "Download as audio only (MP3)")
	rootCmd.Flags().StringVar(&startTime, "start", "", "Start time for clipping")
	rootCmd.Flags().StringVar(&endTime, "end", "", "End time for clipping (e.g., 01:45)")
}

