package controllers

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"wishix/server/internal/models"
	"wishix/server/internal/services"
)

type Lists struct {
	service *services.Lists
	auth    *Auth
}

func NewLists(service *services.Lists, auth *Auth) *Lists {
	return &Lists{service: service, auth: auth}
}

type listResponse struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	EventDate     *string   `json:"event_date"`
	EventTimezone string    `json:"event_timezone"`
	Expired       bool      `json:"expired"`
	Visibility    string    `json:"visibility"`
	SharePath     string    `json:"share_path"`
	Revision      int64     `json:"revision"`
	WishCount     int64     `json:"wish_count"`
	CreatedAt     time.Time `json:"created_at"`
}

func listDate(date *time.Time) *string {
	if date == nil {
		return nil
	}
	value := date.Format("2006-01-02")
	return &value
}
func privateList(list *models.Wishlist) listResponse {
	path := ""
	if list.Visibility == "public" {
		path = "/" + list.ID
	} else if list.Visibility == "link" {
		path = "/" + list.ID + "/" + list.ShareToken
	}
	return listResponse{ID: list.ID, Title: list.Title, Description: list.Description, EventDate: listDate(list.EventDate), EventTimezone: list.EventTimezone, Expired: services.ListExpired(list, time.Now()), Visibility: list.Visibility, SharePath: path, Revision: list.Revision, WishCount: list.WishCount, CreatedAt: list.CreatedAt}
}
func privateLists(lists []models.Wishlist) []listResponse {
	result := make([]listResponse, 0, len(lists))
	for i := range lists {
		result = append(result, privateList(&lists[i]))
	}
	return result
}
func privateWishes(wishes []models.Wish) []wishResponse {
	result := make([]wishResponse, 0, len(wishes))
	for i := range wishes {
		result = append(result, privateWish(&wishes[i]))
	}
	return result
}

func (c *Lists) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/lists", c.authorized(c.list))
	mux.HandleFunc("POST /api/lists", c.authorized(c.create))
	mux.HandleFunc("GET /api/lists/{id}", c.authorized(c.get))
	mux.HandleFunc("PUT /api/lists/{id}", c.authorized(c.update))
	mux.HandleFunc("DELETE /api/lists/{id}", c.authorized(c.delete))
	mux.HandleFunc("POST /api/lists/{id}/copy", c.authorized(c.copy))
	mux.HandleFunc("POST /api/lists/{id}/share-link", c.authorized(c.rotate))
	mux.HandleFunc("POST /api/lists/{id}/wishes", c.authorized(c.add))
	mux.HandleFunc("DELETE /api/lists/{id}/wishes", c.authorized(c.clear))
	mux.HandleFunc("DELETE /api/lists/{id}/wishes/{wishID}", c.authorized(c.remove))
	mux.HandleFunc("PUT /api/lists/{id}/wishes/{wishID}/received", c.authorized(c.setReceived))
	mux.HandleFunc("GET /api/wishes/{id}/lists", c.authorized(c.memberships))
	mux.HandleFunc("PUT /api/wishes/{id}/lists", c.authorized(c.setMemberships))
	mux.HandleFunc("POST /api/public/lists/view", c.public(c.shared))
	mux.HandleFunc("POST /api/public/lists/image", c.public(c.sharedImage))
	mux.HandleFunc("POST /api/public/lists/guest", c.public(c.guest))
	mux.HandleFunc("POST /api/public/lists/guest-image", c.public(c.guestImage))
	mux.HandleFunc("GET /api/public/profiles/{username}", c.profile)
	mux.HandleFunc("GET /api/public/profiles/{username}/wishes", c.collection)
	mux.HandleFunc("GET /api/public/profiles/{username}/wishes/{wishID}/image", c.collectionImage)
	mux.HandleFunc("GET /api/profile/collection", c.authorized(c.collectionSetting))
	mux.HandleFunc("PUT /api/profile/collection", c.authorized(c.setCollectionSetting))
	mux.HandleFunc("GET /api/profile/name", c.authorized(c.getProfileName))
	mux.HandleFunc("PUT /api/profile/name", c.authorized(c.setProfileName))
	mux.HandleFunc("GET /api/search/people", c.authorized(c.searchPeople))
	mux.HandleFunc("GET /{username}/wishlist/shared", c.sharePage)
	mux.HandleFunc("GET /{username}/wishlist/shared/{$}", c.sharePage)
	mux.HandleFunc("GET /u/{username}/wishlist/{id}", c.guestPage)
	mux.HandleFunc("GET /u/{username}/wishlist/{id}/{token}", c.guestPage)
}

var shareUsername = regexp.MustCompile(`^[A-Za-z0-9_]{3,30}$`)

// The front proxy serves the SPA when this returns X-Accel-Redirect.
// Do not log the request URL: it contains a bearer capability.
func (c *Lists) sharePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	if !shareUsername.MatchString(r.PathValue("username")) {
		http.NotFound(w, r)
		return
	}
	values := r.URL.Query()["token"]
	if len(values) != 1 {
		w.Header().Set("X-Accel-Redirect", "/_shared_index")
		return
	}
	list, username, err := c.service.ResolveShare(r.Context(), values[0])
	if err != nil {
		if services.ListErrorCode(err) != "list_not_found" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("X-Accel-Redirect", "/_shared_index")
		return
	}
	if list.Visibility == "public" {
		w.Header().Del("X-Robots-Tag")
	}
	location := guestPath(username, list, values[0])
	w.Header().Set("Location", location)
	if list.Visibility == "public" {
		// Visibility can change back; a permanent redirect would strand the old link.
		w.WriteHeader(http.StatusFound)
	} else {
		w.WriteHeader(http.StatusMovedPermanently)
	}
}

func guestPath(username string, list *models.Wishlist, token string) string {
	path := "/u/" + url.PathEscape(username) + "/wishlist/" + list.ID
	if list.Visibility == "link" {
		path += "/" + token
	}
	return path
}

func (c *Lists) guestPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	if !shareUsername.MatchString(r.PathValue("username")) {
		http.NotFound(w, r)
		return
	}
	list, username, err := c.service.ResolveGuest(r.Context(), r.PathValue("id"), r.PathValue("token"))
	if err != nil {
		if services.ListErrorCode(err) != "list_not_found" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("X-Accel-Redirect", "/_shared_index")
		return
	}
	if list.Visibility == "public" {
		w.Header().Del("X-Robots-Tag")
	}
	canonical := guestPath(username, list, r.PathValue("token"))
	if r.URL.Path != canonical || r.URL.RawQuery != "" {
		w.Header().Set("Location", canonical)
		if list.Visibility == "public" && r.PathValue("token") != "" {
			w.WriteHeader(http.StatusFound)
		} else {
			w.WriteHeader(http.StatusMovedPermanently)
		}
		return
	}
	w.Header().Set("X-Accel-Redirect", "/_shared_index")
}
func readListJSON(r *http.Request, input any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(input) != nil || d.Decode(new(any)) != io.EOF {
		return services.ErrInvalidInput
	}
	return nil
}
func (c *Lists) authorized(handler func(http.ResponseWriter, *http.Request, string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			if !c.mutationHeaders(w, r) {
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
		}
		user, err := c.auth.service.Current(r.Context(), c.auth.token(r))
		if err != nil {
			c.auth.failure(w, err)
			return
		}
		if err := handler(w, r, user.ID); err != nil {
			c.failure(w, err)
		}
	}
}
func (c *Lists) mutationHeaders(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Origin") != c.auth.config.Origin || r.Header.Get("X-Wishix-Request") != "1" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		c.auth.write(w, 403, map[string]string{"error": "forbidden"})
		return false
	}
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media != "application/json" {
		c.auth.write(w, 415, map[string]string{"error": "invalid_input"})
		return false
	}
	return true
}
func (c *Lists) list(w http.ResponseWriter, r *http.Request, owner string) error {
	lists, err := c.service.List(r.Context(), owner)
	if err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]any{"lists": privateLists(lists)})
	return nil
}
func (c *Lists) get(w http.ResponseWriter, r *http.Request, owner string) error {
	list, wishes, err := c.service.Get(r.Context(), owner, r.PathValue("id"))
	if err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]any{"list": privateList(list), "wishes": privateWishes(wishes)})
	return nil
}
func (c *Lists) create(w http.ResponseWriter, r *http.Request, owner string) error {
	var input services.ListInput
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	list, err := c.service.Create(r.Context(), owner, input)
	if err != nil {
		return err
	}
	c.auth.write(w, 201, map[string]any{"list": privateList(list)})
	return nil
}
func (c *Lists) update(w http.ResponseWriter, r *http.Request, owner string) error {
	var input services.ListInput
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	list, err := c.service.Update(r.Context(), owner, r.PathValue("id"), input)
	if err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]any{"list": privateList(list)})
	return nil
}
func (c *Lists) destroy(w http.ResponseWriter, r *http.Request, owner string, clear bool) error {
	var input struct {
		Revision int64  `json:"revision"`
		Message  string `json:"message"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	if err := c.service.Destroy(r.Context(), owner, r.PathValue("id"), input.Revision, clear, input.Message); err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]string{"status": "ok"})
	return nil
}
func (c *Lists) delete(w http.ResponseWriter, r *http.Request, owner string) error {
	return c.destroy(w, r, owner, false)
}
func (c *Lists) clear(w http.ResponseWriter, r *http.Request, owner string) error {
	return c.destroy(w, r, owner, true)
}
func (c *Lists) copy(w http.ResponseWriter, r *http.Request, owner string) error {
	var input struct {
		Title string `json:"title"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	list, err := c.service.Copy(r.Context(), owner, r.PathValue("id"), input.Title)
	if err != nil {
		return err
	}
	c.auth.write(w, 201, map[string]any{"list": privateList(list)})
	return nil
}
func (c *Lists) rotate(w http.ResponseWriter, r *http.Request, owner string) error {
	var input struct {
		Revision int64 `json:"revision"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	list, err := c.service.Rotate(r.Context(), owner, r.PathValue("id"), input.Revision)
	if err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]any{"list": privateList(list)})
	return nil
}
func (c *Lists) add(w http.ResponseWriter, r *http.Request, owner string) error {
	var input struct {
		WishIDs []string `json:"wish_ids"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	if err := c.service.Add(r.Context(), owner, r.PathValue("id"), input.WishIDs); err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]string{"status": "ok"})
	return nil
}
func (c *Lists) remove(w http.ResponseWriter, r *http.Request, owner string) error {
	if err := c.service.Remove(r.Context(), owner, r.PathValue("id"), r.PathValue("wishID")); err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]string{"status": "ok"})
	return nil
}
func (c *Lists) setReceived(w http.ResponseWriter, r *http.Request, owner string) error {
	var input struct {
		Received *bool `json:"received"`
		Revision int64 `json:"revision"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	if input.Received == nil {
		return services.ErrInvalidInput
	}
	if err := c.service.SetReceived(r.Context(), owner, r.PathValue("id"), r.PathValue("wishID"), input.Revision, *input.Received); err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]string{"status": "ok"})
	return nil
}
func (c *Lists) memberships(w http.ResponseWriter, r *http.Request, owner string) error {
	ids, version, err := c.service.Memberships(r.Context(), owner, r.PathValue("id"))
	if err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]any{"list_ids": ids, "version": version})
	return nil
}
func (c *Lists) setMemberships(w http.ResponseWriter, r *http.Request, owner string) error {
	var input struct {
		ListIDs []string `json:"list_ids"`
		Version int64    `json:"version"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	if err := c.service.SetMemberships(r.Context(), owner, r.PathValue("id"), input.ListIDs, input.Version); err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]string{"status": "ok"})
	return nil
}
func (c *Lists) failure(w http.ResponseWriter, err error) {
	code := services.ListErrorCode(err)
	status := 400
	if code == "list_not_found" || code == "wish_not_found" {
		status = 404
	} else if code == "list_changed" {
		status = 409
	} else if code == "unauthorized" {
		status = 401
	} else if code == "unavailable" {
		status = 503
		c.auth.logger.Error("list operation unavailable")
	}
	c.auth.write(w, status, map[string]string{"error": code})
}

// Public DTOs deliberately omit owner IDs, sessions, email, revisions and private image URLs.
type publicListResponse struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	EventDate   *string `json:"event_date"`
	Expired     bool    `json:"expired"`
	Visibility  string  `json:"visibility"`
	WishCount   int64   `json:"wish_count"`
	SharePath   string  `json:"share_path,omitempty"`
}

func publicList(list *models.Wishlist, link bool) publicListResponse {
	expired := services.ListExpired(list, time.Now())
	result := publicListResponse{ID: list.ID, Title: list.Title, Description: list.Description, EventDate: listDate(list.EventDate), Expired: expired, Visibility: list.Visibility, WishCount: list.WishCount}
	if expired {
		result.WishCount = 0
	}
	if link && list.Visibility == "public" {
		result.SharePath = "/" + list.ID
	}
	return result
}

func (c *Lists) guest(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	list, username, wishes, err := c.service.Guest(r.Context(), input.ID, input.Token)
	if err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]any{"list": publicList(list, false), "username": username, "wishes": publicWishes(wishes)})
	return nil
}
func (c *Lists) guestImage(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		ID     string `json:"id"`
		Token  string `json:"token"`
		WishID string `json:"wish_id"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	image, err := c.service.GuestImage(r.Context(), input.ID, input.Token, input.WishID)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(image.Data)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(image.Data)
	return nil
}

type publicWishResponse struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Price            *string  `json:"price"`
	Currency         string   `json:"currency"`
	Priority         string   `json:"priority"`
	ShopLinks        []string `json:"shop_links"`
	HasImage         bool     `json:"has_image"`
	Reserved         bool     `json:"reserved"`
	GuestRecoverable bool     `json:"guest_recoverable"`
	Received         bool     `json:"received"`
}

func (c *Lists) public(handler func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Robots-Tag", "noindex, noarchive")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if !c.mutationHeaders(w, r) {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := handler(w, r); err != nil {
			c.failure(w, err)
		}
	}
}
func (c *Lists) shared(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Token string `json:"token"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	list, username, wishes, err := c.service.Shared(r.Context(), input.Token)
	if err != nil {
		return err
	}
	result := publicWishes(wishes)
	c.auth.write(w, 200, map[string]any{"list": publicList(list, false), "username": username, "wishes": result})
	return nil
}
func publicWishes(wishes []models.Wish) []publicWishResponse {
	result := make([]publicWishResponse, 0, len(wishes))
	for _, wish := range wishes {
		links := make([]string, 0, len(wish.ShopLinks))
		for _, link := range wish.ShopLinks {
			links = append(links, link.URL)
		}
		result = append(result, publicWishResponse{ID: wish.ID, Title: wish.Title, Description: wish.Description, Price: wish.Price, Currency: wish.Currency, Priority: wish.Priority, ShopLinks: links, HasImage: wish.ImageVersion != "", Reserved: wish.Reserved, GuestRecoverable: wish.GuestRecoverable, Received: wish.Received})
	}
	return result
}
func (c *Lists) sharedImage(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Token  string `json:"token"`
		WishID string `json:"wish_id"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	image, err := c.service.SharedImage(r.Context(), input.Token, input.WishID)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(image.Data)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(image.Data)
	return nil
}
func (c *Lists) profile(w http.ResponseWriter, r *http.Request) {
	username, displayName, enabled, lists, err := c.service.PublicProfile(r.Context(), r.PathValue("username"))
	if err != nil {
		c.failure(w, err)
		return
	}
	result := make([]publicListResponse, 0, len(lists))
	for i := range lists {
		result = append(result, publicList(&lists[i], true))
	}
	c.auth.write(w, 200, map[string]any{"username": username, "display_name": displayName, "show_all_wishes": enabled, "lists": result})
}

func (c *Lists) searchPeople(w http.ResponseWriter, r *http.Request, owner string) error {
	people, cursor, err := c.service.SearchPeople(r.Context(), owner, r.URL.Query().Get("q"), r.URL.Query().Get("after"))
	if err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]any{"people": people, "next_cursor": cursor})
	return nil
}
func (c *Lists) getProfileName(w http.ResponseWriter, r *http.Request, owner string) error {
	name, public, err := c.service.ProfileName(r.Context(), owner)
	if err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]any{"display_name": name, "show_display_name": public})
	return nil
}
func (c *Lists) setProfileName(w http.ResponseWriter, r *http.Request, owner string) error {
	var input struct {
		Name   *string `json:"display_name"`
		Public *bool   `json:"show_display_name"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	if input.Name == nil || input.Public == nil {
		return services.ErrInvalidInput
	}
	if err := c.service.SetProfileName(r.Context(), owner, *input.Name, *input.Public); err != nil {
		return err
	}
	name, public, err := c.service.ProfileName(r.Context(), owner)
	if err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]any{"display_name": name, "show_display_name": public})
	return nil
}

func (c *Lists) collectionSetting(w http.ResponseWriter, r *http.Request, owner string) error {
	enabled, err := c.service.CollectionSetting(r.Context(), owner)
	if err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]bool{"show_all_wishes": enabled})
	return nil
}
func (c *Lists) setCollectionSetting(w http.ResponseWriter, r *http.Request, owner string) error {
	var input struct {
		Show *bool `json:"show_all_wishes"`
	}
	if err := readListJSON(r, &input); err != nil {
		return err
	}
	if input.Show == nil {
		return services.ErrInvalidInput
	}
	if err := c.service.SetCollectionSetting(r.Context(), owner, *input.Show); err != nil {
		return err
	}
	c.auth.write(w, 200, map[string]bool{"show_all_wishes": *input.Show})
	return nil
}
func (c *Lists) collection(w http.ResponseWriter, r *http.Request) {
	wishes, err := c.service.PublicCollection(r.Context(), r.PathValue("username"))
	if err != nil {
		c.failure(w, err)
		return
	}
	result := make([]map[string]any, 0, len(wishes))
	for _, wish := range publicWishes(wishes) {
		result = append(result, map[string]any{"id": wish.ID, "title": wish.Title, "description": wish.Description, "price": wish.Price, "currency": wish.Currency, "priority": wish.Priority, "shop_links": wish.ShopLinks, "has_image": wish.HasImage})
	}
	c.auth.write(w, 200, map[string]any{"username": strings.ToLower(r.PathValue("username")), "wishes": result})
}
func (c *Lists) collectionImage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	image, err := c.service.PublicCollectionImage(r.Context(), r.PathValue("username"), r.PathValue("wishID"))
	if err != nil {
		c.failure(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(image.Data)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(image.Data)
}
