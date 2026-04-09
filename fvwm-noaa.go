package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/gographics/imagick.v3/imagick"
)

//Enter a location for the forcast
//Could get lat/long from the station if I wanted
var latitude string = "38.5743"
var longitude string = "-121.4342"
var station string = "KSAC"

//constants for conversion
//kph to mph
const kphtomph float64 = 0.62137

//Pascals to mmHg
const patommhg float64 = 0.00750062

type Forecast struct {
	name          string
	startTime     string
	isDaytime     bool
	temp          float64
	precip        string
	windSpeed     string
	windDirection string
	iconURL       string
	shortforecast string
	details       string
}

type Observation struct {
	Properties struct {
		Timestamp       string
		TextDescription string
		Icon            string
		Temperature     struct {
			UnitCode       string  `json:"unitCode"`
			Value          float64 `json:"value"`
			QualityControl string  `json:"qualityControl"`
		}
		Dewpoint struct {
			UnitCode       string  `json:"unitCode"`
			Value          float64 `json:"value"`
			QualityControl string  `json:"qualityControl"`
		}
		WindDirection struct {
			UnitCode       string  `json:"unitCode"`
			Value          float64 `json:"value"`
			QualityControl string  `json:"qualityControl"`
		}
		WindSpeed struct {
			UnitCode       string  `json:"unitCode"`
			Value          float64 `json:"value"`
			QualityControl string  `json:"qualityControl"`
		}
		WindGust struct {
			UnitCode       string  `json:"unitCode"`
			Value          float64 `json:"value"`
			QualityControl string  `json:"qualityControl"`
		}
		BarometricPressure struct {
			UnitCode       string  `json:"unitCode"`
			Value          float64 `json:"value"`
			QualityControl string  `json:"qualityControl"`
		}
		RelativeHumidity struct {
			UnitCode       string  `json:"unitCode"`
			Value          float64 `json:"value"`
			QualityControl string  `json:"qualityControl"`
		}
	}
}

var location string
var daily_data []Forecast
var hourly_data []Forecast
var current_data Observation

func main() {
	os.Setenv("FVWM_USERDIR", "/home/bob/.fvwm")
	//var url string = "https://api.weather.gov/stations/" + station
	var current_obs_url string = "https://api.weather.gov/stations/" + station + "/observations/latest?require_qc=false"
	var url string = "https://api.weather.gov/points/" + latitude + "," + longitude
	//Data from the base url and then data from the forecast urls off the base url
	var station_data interface{}
	var forecast_data interface{}
	var forecast_hourly_data interface{}
	var observation_data interface{}
	//start up imagemagick for annotating temperature data on the images
	imagick.Initialize()
	defer imagick.Terminate()

	//Get the station data
	err := get_json(url, &station_data)
	if err != nil {
		fmt.Printf("get_forecast error: %v\n", err)
	}
	m := station_data.(map[string]interface{})

	//This gets the actual forecast URL from the station
	for k, v := range m {
		if k == "properties" {
			data := v.(map[string]interface{})
			//get the location first
			d := data["relativeLocation"].(map[string]interface{})
			loc := d["properties"].(map[string]interface{})
			location = loc["city"].(string) + ",\\ " + loc["state"].(string)
			//Get the daily forecast data, day and night, build the menu and generate button images
			forecast_url := data["forecast"].(string)
			err = get_json(forecast_url, &forecast_data)
			if err != nil {
				fmt.Printf("get forecast data error: %v\n", err)
			}
			raw_daily_data := parse_forecast(forecast_data)
			parse_daily_data(raw_daily_data, &daily_data)
			generate_daily_menu(daily_data)

			//Do the same for the hourly forecast data
			forecast_hourly_url := data["forecastHourly"].(string)
			err = get_json(forecast_hourly_url, &forecast_hourly_data)
			if err != nil {
				fmt.Printf("get forecast data error: %v\n", err)
			}
			raw_hourly_data := parse_forecast(forecast_hourly_data)
			parse_daily_data(raw_hourly_data, &hourly_data)
			generate_hourly_menu(hourly_data)
			//Finally get the current observations using the station url
			//which is of course in a completely different format
			err = get_json(current_obs_url, &observation_data)
			if err != nil {
				fmt.Printf("get observation data error: %v\n", err)
			}
			parse_observation(observation_data)
		}
	}
}

//Current weather observation data from a named station
func parse_observation(raw interface{}) {
	data, err := json.Marshal(raw)
	if err != nil {
		fmt.Printf("marshal error: %s", err)
	}
	err = json.Unmarshal(data, &current_data)
	if err != nil {
		fmt.Printf("error unmarshaling: %s", err)
	}
	//Get the icon and set the button image with current temp in deg F
	imagepath := save_image(current_data.Properties.Icon, true)
	tempF := current_data.Properties.Temperature.Value*1.8 + 32
	temp := strconv.FormatFloat(tempF, 'f', 1, 64)
	set_button_image_current(imagepath, temp)
	//Make a menu with the current observations
	fvwm("DestroyMenu CurrentWeather")
	hourmin := hourmin(current_data.Properties.Timestamp)
	addstring := "AddToMenu\\ CurrentWeather\\ " + strconv.Quote(hourmin) + ` Title`
	fvwm(addstring)
	fvwm_no_popup("Temperature \t" + strconv.FormatFloat(current_data.Properties.Temperature.Value*1.8+32, 'f', 1, 64) + " F")
	fvwm_no_popup("Conditions \t" + current_data.Properties.TextDescription)
	fvwm_no_popup("Dewpoint \t" + strconv.FormatFloat(current_data.Properties.Dewpoint.Value*1.8+32, 'f', 1, 64) + " F")
	fvwm_no_popup("Wind Speed \t" + strconv.FormatFloat(current_data.Properties.WindSpeed.Value*kphtomph, 'f', 1, 64) + " mph")
	fvwm_no_popup("Wind Direction \t" + strconv.FormatFloat(current_data.Properties.WindDirection.Value, 'f', 0, 64) + " deg")
	fvwm_no_popup("Wind Gust \t" + strconv.FormatFloat(current_data.Properties.WindGust.Value*kphtomph, 'f', 1, 64) + " mph")
	fvwm_no_popup("Pressure \t" + strconv.FormatFloat(current_data.Properties.BarometricPressure.Value*patommhg, 'f', 0, 64) + " mmHg")
	fvwm_no_popup("Humidity \t" + strconv.FormatFloat(current_data.Properties.RelativeHumidity.Value, 'f', 0, 64) + " %")
}

//Parses the JSON from the forecast URL to find the daily forecasts
func parse_forecast(raw interface{}) []interface{} {
	forecast := raw.(map[string]interface{})
	for k, v := range forecast {
		if k == "properties" {
			data := v.(map[string]interface{})
			for kk, vv := range data {
				if kk == "periods" {
					daily_data := vv.([]interface{})
					return daily_data
				}
			}
		}
	}
	return nil
}

//Parses the array of forecast data
func parse_daily_data(data interface{}, output *[]Forecast) {
	forecast := data.([]interface{})
	for _, val := range forecast {
		daily := val.(map[string]interface{})
		tmp := daily["probabilityOfPrecipitation"]
		precip := tmp.(map[string]interface{})
		percent := precip["value"]
		if percent == nil {
			percent = "0"
		} else {
			percent = strconv.Itoa(int(percent.(float64)))
		}
		today := Forecast{
			daily["name"].(string),
			daily["startTime"].(string),
			daily["isDaytime"].(bool),
			daily["temperature"].(float64),
			percent.(string),
			daily["windSpeed"].(string),
			daily["windDirection"].(string),
			daily["icon"].(string),
			daily["shortForecast"].(string),
			daily["detailedForecast"].(string),
		}
		*output = append(*output, today)
	}
}

//Outputs the Hourly Forecast menus
func generate_hourly_menu(data []Forecast) {
	day := short_day(data[0].startTime)
	start_day := day
	fvwm("DestroyMenu " + day)
	fvwm("AddToMenu " + day + " " + day + ` Title`)
	nop()
	for _, val := range data {
		current_day := short_day(val.startTime)
		//If it's a new day start a new menu
		if day != current_day {
			day = current_day
			//We've wrapped around to the next week
			if day == start_day {
				return
			}
			fvwm("DestroyMenu " + day)
			fvwm("AddToMenu " + day + " " + day + ` Title`)
		}
		var temp string = strconv.Itoa(int(val.temp))
		imagepath := save_image(val.iconURL, val.isDaytime)
		hour := hour(val.startTime)
		var sb strings.Builder
		sb.WriteString("%")
		sb.WriteString(imagepath)
		sb.WriteString("%")
		sb.WriteString(hour)
		sb.WriteString(" ")
		sb.WriteString(temp)
		sb.WriteString(" ")
		sb.WriteString(val.precip)
		sb.WriteString(" ")
		sb.WriteString(val.shortforecast)
		fvwm_no_popup(sb.String())
	}
}

//Ouputs the Forecast menu in fvwm3 syntax
func generate_daily_menu(data []Forecast) {
	//The header
	fvwm("DestroyMenu Forecast")
	//very particular formatting in this line
	fvwm("AddToMenu Forecast " + strconv.Quote(location) + ` Title`)
	fvwm_no_popup("Date  Min/Max  Conditions ")
	//nop()
	var max string        //max daily temperature
	var imagepath string  //local path to weather icon
	var dateString string //day month date
	var shortday string   //3 letter day name
	//If the 0th element has isDaytime true then start at 0
	//otherwise use 0 for the current weather and start at 1
	var day = data[0].isDaytime
	for i, val := range data {
		//There are daytime and nighttime forcasts for each day
		//The max temp is during the day and the min temp is at night
		//Since I only want one line per day write it out during the night forcast parse
		if !day && i != 0 {
			var min string = strconv.Itoa(int(val.temp))
			var sb strings.Builder
			sb.WriteString("%")
			sb.WriteString(imagepath)
			sb.WriteString("%")
			sb.WriteString(dateString)
			sb.WriteString(" ")
			sb.WriteString(min)
			sb.WriteString("/")
			sb.WriteString(max)
			sb.WriteString(" ")
			sb.WriteString(val.shortforecast)
			fvwm_popup(sb.String(), shortday)
			//button image numbers start at 1 not 0
			set_button_image(imagepath, shortday, min, max, i/2+1)
			day = !day
		} else {
			max = strconv.Itoa(int(val.temp))
			icon := val.iconURL
			imagepath = save_image(icon, val.isDaytime)
			dateString = parse_time(val.startTime)
			shortday = short_day(val.startTime)
			day = !day
		}
	}
}

//Writes the temperature on the current weather icon and saves it
func set_button_image_current(imagepath string, temp string) {
	mw := imagick.MagickWand{}
	defer mw.Destroy()
	savepath := "/tmp/current.png"
	//Make it fit the button
	imagick.ConvertImageCommand([]string{
		"magick", "-quiet", imagepath, "-resize", "57", savepath,
	})
	imagick.ConvertImageCommand([]string{
		"magick", "-quiet",
		"-pointsize", "18",
		"-fill", "#330000",
		savepath,
		"-annotate", "+20+50", temp,
		savepath,
	})
	fvwm("SendToModule WeatherButtons ChangeButton Current Icon " + savepath)
}

//Writes the min/max temperature on the image and puts it into the FvwmButton container
func set_button_image(imagepath string, day string, min string, max string, index int) {
	mw := imagick.MagickWand{}
	defer mw.Destroy()
	savepath := "/tmp/" + strconv.Itoa(index) + ".png"
	top := "text +1+12 " + day
	bottom := min + "/" + max
	imagick.ConvertImageCommand([]string{
		"magick", "-quiet", imagepath, "-resize", "55", savepath,
	})
	//add the day on top
	imagick.ConvertImageCommand([]string{
		"magick", "-quiet",
		"-pointsize", "18",
		"-fill", "black",
		savepath,
		"-draw", top,
		savepath,
	})
	//add the min/max temp on the bottom
	imagick.ConvertImageCommand([]string{
		"magick", "-quiet",
		"-pointsize", "18",
		"-fill", "black",
		savepath,
		"-annotate", "+1+50", bottom,
		savepath,
	})
	//send the icon to the correct button station
	fvwm("SendToModule WeatherButtons ChangeButton Day" + strconv.Itoa(index) + " Icon " + savepath)
}

//Helper functions to pass commands to FvwmCommand
func fvwm(line string) {
	line = "echo -e " + line + " | /usr/bin/FvwmCommand -c"
	//println(line)
	_, err := exec.Command("/bin/bash", "-c", line).Output()
	if err != nil {
		fmt.Printf("command exec error in fvwm(): %s\n", err)
	}
}

//not working, should just put a line
func nop() {
	_, err := exec.Command("/bin/bash", "-c", "+ "+"\\ "+"` Nop` | /usr/bin/FvwmCommand -c").Output()
	if err != nil {
		fmt.Printf("command exec error in nop(): %s\n", err)
	}
}
func fvwm_no_popup(line string) {
	re := regexp.MustCompile(`\s`)
	line = re.ReplaceAllString(line, "\\ ")
	line = "echo -e '+ " + line + "' | /usr/bin/FvwmCommand -c"
	//println(line)
	_, err := exec.Command("/bin/bash", "-c", line).Output()
	if err != nil {
		fmt.Printf("command exec error in fvwm_no_popup(): %s\n", err)
	}
}

func fvwm_popup(line string, day string) {
	re := regexp.MustCompile(`\s`)
	line = re.ReplaceAllString(line, "\\ ")
	line = "echo -e '+ " + line + "' Popup " + day + " | /usr/bin/FvwmCommand -c"
	//println(line)
	_, err := exec.Command("/bin/bash", "-c", line).Output()
	if err != nil {
		fmt.Printf("command exec error in fvwm_popup(): %s\n", err)
	}

	/*  Doesn't work, can't figure out how to escape spaces correctly
	com := exec.Command("FvwmCommand", "-c")
	pipe, err := fvwm.StdoutPipe()
	if err != nil {
		fmt.Printf("error creating pipe: %s", err)
	}
	defer pipe.Close()

	com.Stdin = pipe
	fvwm.Start()
	_, err = com.Output()
	if err != nil {
		fmt.Printf("error running command: %s", err)
	}
	*/
}

//Parse a date-time string and return a string in the format Dayname Monthname Date
func parse_time(datetime string) string {
	layout := "2006-01-02T15:04:05-07:00"
	t, err := time.Parse(layout, datetime)
	if err != nil {
		fmt.Printf("error parsing datetime: %s", err)
		return ""
	}
	var formatted string = t.Weekday().String()[0:3] + " " + t.Month().String()[0:3] + " " + strconv.Itoa(t.Day())
	return formatted
}

//Parse a date-time string and return a 3 letter day name
func short_day(datetime string) string {
	layout := "2006-01-02T15:04:05-07:00"
	t, err := time.Parse(layout, datetime)
	if err != nil {
		fmt.Printf("error parsing datetime: %s", err)
		return ""
	}
	var day string = t.Weekday().String()[0:3]
	return day
}

//Parse a date-time string and return an hour with :00
func hour(datetime string) string {
	layout := "2006-01-02T15:04:05-07:00"
	t, err := time.Parse(layout, datetime)
	if err != nil {
		fmt.Printf("error parsing datetime: %s", err)
		return ""
	}
	var hour string = strconv.Itoa(t.Hour())
	return hour + ":00"
}

//Parse a date-time string and return an escaped day month date H:MM string
func hourmin(datetime string) string {
	layout := "2006-01-02T15:04:05-07:00"
	t, err := time.Parse(layout, datetime)
	if err != nil {
		fmt.Printf("error parsing datetime: %s", err)
		return ""
	}
	var hour string = strconv.Itoa(t.Hour())
	var min string = strconv.Itoa(t.Minute())
	var formatted string = t.Weekday().String()[0:3] + "\\ " + t.Month().String()[0:3] + "\\ " + strconv.Itoa(t.Day()) + "\\ " + hour + ":" + min
	return formatted
}

//Save the NOAA weather icons in ~/fvwm/images if it doesn't exist
func save_image(imageURL string, isDaytime bool) string {
	//download the image
	image, err := http.Get(imageURL)
	if err != nil {
		fmt.Printf("image download error: %s", err)
	}
	defer image.Body.Close()

	//generate the correct full path
	filename := path.Base(imageURL)
	//home := os.Getenv("HOME")
	var tag string = "night"
	if isDaytime {
		tag = "day"
	}
	imagename := "/home/bob/fvwm/images/" + filename + "-" + tag + ".png"

	//check to see if we already have this image
	/*
		_, err = os.Stat(imagename)
		if err == nil {
			return imagename
		}
	*/

	//make the file to store the image if it doesn't exist
	file, err := os.Create(imagename)
	if err != nil {
		fmt.Printf("file creation error: %s\n", err)
		return ""
	}
	defer file.Close()

	//copy the image data to the file and return the path
	_, err = io.Copy(file, image.Body)
	if err != nil {
		fmt.Printf("image file copy fail: %s", err)
		return ""
	}
	return imagename
}

//Get the JSON at the given URL and fill in the interface{}
func get_json(url string, result interface{}) error {
	println(url)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("URL get error %q: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET status error: %s", resp.Status)
	}
	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		return fmt.Errorf("JSON decode error: %v", err)
	}
	return nil
}
