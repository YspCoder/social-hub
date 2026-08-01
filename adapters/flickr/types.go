package flickr

import (
	"bytes"
	"encoding/json"
	"strconv"
)

type scalar string

func (value *scalar) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*value = ""
		return nil
	}
	if bytes.Equal(data, []byte("true")) {
		*value = "true"
		return nil
	}
	if bytes.Equal(data, []byte("false")) {
		*value = "false"
		return nil
	}
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		*value = scalar(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	*value = scalar(number.String())
	return nil
}

func (value scalar) String() string { return string(value) }

func (value scalar) Int64() (int64, bool) {
	parsed, err := strconv.ParseInt(string(value), 10, 64)
	return parsed, err == nil
}

func (value scalar) Int() (int, bool) {
	parsed, err := strconv.Atoi(string(value))
	return parsed, err == nil
}

func (value scalar) Bool() bool { return value == "1" || value == "true" }

type content struct {
	Text string `json:"_content"`
}

// Person is a Flickr member returned by flickr.people.getInfo.
type Person struct {
	NSID       string  `json:"nsid"`
	IsPro      scalar  `json:"ispro"`
	IconServer scalar  `json:"iconserver"`
	IconFarm   scalar  `json:"iconfarm"`
	Username   content `json:"username"`
	RealName   content `json:"realname"`
	Location   content `json:"location"`
	PhotosURL  content `json:"photosurl"`
	ProfileURL content `json:"profileurl"`
	Photos     struct {
		FirstDate scalar `json:"firstdate"`
		Count     scalar `json:"count"`
	} `json:"photos"`
}

// Photo contains Flickr photo metadata.
type Photo struct {
	ID             string `json:"id"`
	Secret         string `json:"secret"`
	Server         string `json:"server"`
	Farm           scalar `json:"farm"`
	OriginalSecret string `json:"originalsecret"`
	OriginalFormat string `json:"originalformat"`
	Media          string `json:"media"`
	IsFavorite     scalar `json:"isfavorite"`
	License        scalar `json:"license"`
	SafetyLevel    scalar `json:"safety_level"`
	Rotation       scalar `json:"rotation"`
	Views          scalar `json:"views"`
	Owner          struct {
		NSID     string `json:"nsid"`
		Username string `json:"username"`
		RealName string `json:"realname"`
		Location string `json:"location"`
	} `json:"owner"`
	Title       content `json:"title"`
	Description content `json:"description"`
	Visibility  struct {
		IsPublic scalar `json:"ispublic"`
		IsFriend scalar `json:"isfriend"`
		IsFamily scalar `json:"isfamily"`
	} `json:"visibility"`
	Dates struct {
		Posted     scalar `json:"posted"`
		Taken      string `json:"taken"`
		LastUpdate scalar `json:"lastupdate"`
	} `json:"dates"`
	Comments content `json:"comments"`
	Tags     struct {
		Tag []struct {
			ID      string `json:"id"`
			Author  string `json:"author"`
			Raw     string `json:"raw"`
			Content string `json:"_content"`
		} `json:"tag"`
	} `json:"tags"`
	URLs struct {
		URL []struct {
			Type    string `json:"type"`
			Content string `json:"_content"`
		} `json:"url"`
	} `json:"urls"`
}

// PhotoSummary is a Flickr photo-list row with requested extras.
type PhotoSummary struct {
	ID          string  `json:"id"`
	Owner       string  `json:"owner"`
	Secret      string  `json:"secret"`
	Server      string  `json:"server"`
	Farm        scalar  `json:"farm"`
	Title       string  `json:"title"`
	Description content `json:"description"`
	IsPublic    scalar  `json:"ispublic"`
	IsFriend    scalar  `json:"isfriend"`
	IsFamily    scalar  `json:"isfamily"`
	DateUpload  scalar  `json:"dateupload"`
	LastUpdate  scalar  `json:"lastupdate"`
	OwnerName   string  `json:"ownername"`
	Tags        string  `json:"tags"`
	Views       scalar  `json:"views"`
	Media       string  `json:"media"`
	Width       scalar  `json:"width_o"`
	Height      scalar  `json:"height_o"`
	URLOriginal string  `json:"url_o"`
	URLLarge    string  `json:"url_l"`
	URLMedium   string  `json:"url_c"`
	URLSmall    string  `json:"url_m"`
}

// PhotoComment is a flat Flickr photo comment.
type PhotoComment struct {
	ID         string `json:"id"`
	Author     string `json:"author"`
	AuthorName string `json:"authorname"`
	DateCreate scalar `json:"datecreate"`
	Permalink  string `json:"permalink"`
	Content    string `json:"_content"`
}

// Album is Flickr's photoset representation.
type Album struct {
	ID            string  `json:"id"`
	Owner         string  `json:"owner"`
	Primary       string  `json:"primary"`
	Secret        string  `json:"secret"`
	Server        string  `json:"server"`
	Farm          scalar  `json:"farm"`
	Photos        scalar  `json:"photos"`
	Videos        scalar  `json:"videos"`
	CountViews    scalar  `json:"count_views"`
	CountComments scalar  `json:"count_comments"`
	DateCreate    scalar  `json:"date_create"`
	DateUpdate    scalar  `json:"date_update"`
	Title         content `json:"title"`
	Description   content `json:"description"`
}

// AlbumReference is returned when a photoset is created.
type AlbumReference struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type personResponse struct {
	Person Person `json:"person"`
}

type photoResponse struct {
	Photo Photo `json:"photo"`
}

type photoPage struct {
	Page    scalar         `json:"page"`
	Pages   scalar         `json:"pages"`
	PerPage scalar         `json:"perpage"`
	Total   scalar         `json:"total"`
	Photos  []PhotoSummary `json:"photo"`
}

type photosResponse struct {
	Photos photoPage `json:"photos"`
}

type commentsResponse struct {
	Comments struct {
		PhotoID string         `json:"photo_id"`
		Items   []PhotoComment `json:"comment"`
	} `json:"comments"`
}

type commentResponse struct {
	Comment struct {
		ID string `json:"id"`
	} `json:"comment"`
}

type albumResponse struct {
	Album Album `json:"photoset"`
}

type albumPage struct {
	Page    scalar  `json:"page"`
	Pages   scalar  `json:"pages"`
	PerPage scalar  `json:"perpage"`
	Total   scalar  `json:"total"`
	Albums  []Album `json:"photoset"`
}

type albumsResponse struct {
	Albums albumPage `json:"photosets"`
}

type albumPhotosResponse struct {
	Photos struct {
		ID      string         `json:"id"`
		Owner   string         `json:"owner"`
		Page    scalar         `json:"page"`
		Pages   scalar         `json:"pages"`
		PerPage scalar         `json:"perpage"`
		Total   scalar         `json:"total"`
		Items   []PhotoSummary `json:"photo"`
	} `json:"photoset"`
}

type createAlbumResponse struct {
	Album AlbumReference `json:"photoset"`
}
