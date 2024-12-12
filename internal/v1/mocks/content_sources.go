package mocks

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/osbuild/image-builder/internal/clients/content_sources"
	"github.com/osbuild/image-builder/internal/common"
	"github.com/osbuild/image-builder/internal/tutils"
)

const (
	CentosGPG = "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBFzMWxkBEADHrskpBgN9OphmhRkc7P/YrsAGSvvl7kfu+e9KAaU6f5MeAVyn\nrIoM43syyGkgFyWgjZM8/rur7EMPY2yt+2q/1ZfLVCRn9856JqTIq0XRpDUe4nKQ\n8BlA7wDVZoSDxUZkSuTIyExbDf0cpw89Tcf62Mxmi8jh74vRlPy1PgjWL5494b3X\n5fxDidH4bqPZyxTBqPrUFuo+EfUVEqiGF94Ppq6ZUvrBGOVo1V1+Ifm9CGEK597c\naevcGc1RFlgxIgN84UpuDjPR9/zSndwJ7XsXYvZ6HXcKGagRKsfYDWGPkA5cOL/e\nf+yObOnC43yPUvpggQ4KaNJ6+SMTZOKikM8yciyBwLqwrjo8FlJgkv8Vfag/2UR7\nJINbyqHHoLUhQ2m6HXSwK4YjtwidF9EUkaBZWrrskYR3IRZLXlWqeOi/+ezYOW0m\nvufrkcvsh+TKlVVnuwmEPjJ8mwUSpsLdfPJo1DHsd8FS03SCKPaXFdD7ePfEjiYk\nnHpQaKE01aWVSLUiygn7F7rYemGqV9Vt7tBw5pz0vqSC72a5E3zFzIIuHx6aANry\nGat3aqU3qtBXOrA/dPkX9cWE+UR5wo/A2UdKJZLlGhM2WRJ3ltmGT48V9CeS6N9Y\nm4CKdzvg7EWjlTlFrd/8WJ2KoqOE9leDPeXRPncubJfJ6LLIHyG09h9kKQARAQAB\ntDpDZW50T1MgKENlbnRPUyBPZmZpY2lhbCBTaWduaW5nIEtleSkgPHNlY3VyaXR5\nQGNlbnRvcy5vcmc+iQI3BBMBAgAhBQJczFsZAhsDBgsJCAcDAgYVCAIJCgsDFgIB\nAh4BAheAAAoJEAW1VbOEg8ZdjOsP/2ygSxH9jqffOU9SKyJDlraL2gIutqZ3B8pl\nGy/Qnb9QD1EJVb4ZxOEhcY2W9VJfIpnf3yBuAto7zvKe/G1nxH4Bt6WTJQCkUjcs\nN3qPWsx1VslsAEz7bXGiHym6Ay4xF28bQ9XYIokIQXd0T2rD3/lNGxNtORZ2bKjD\nvOzYzvh2idUIY1DgGWJ11gtHFIA9CvHcW+SMPEhkcKZJAO51ayFBqTSSpiorVwTq\na0cB+cgmCQOI4/MY+kIvzoexfG7xhkUqe0wxmph9RQQxlTbNQDCdaxSgwbF2T+gw\nbyaDvkS4xtR6Soj7BKjKAmcnf5fn4C5Or0KLUqMzBtDMbfQQihn62iZJN6ZZ/4dg\nq4HTqyVpyuzMXsFpJ9L/FqH2DJ4exGGpBv00ba/Zauy7GsqOc5PnNBsYaHCply0X\n407DRx51t9YwYI/ttValuehq9+gRJpOTTKp6AjZn/a5Yt3h6jDgpNfM/EyLFIY9z\nV6CXqQQ/8JRvaik/JsGCf+eeLZOw4koIjZGEAg04iuyNTjhx0e/QHEVcYAqNLhXG\nrCTTbCn3NSUO9qxEXC+K/1m1kaXoCGA0UWlVGZ1JSifbbMx0yxq/brpEZPUYm+32\no8XfbocBWljFUJ+6aljTvZ3LQLKTSPW7TFO+GXycAOmCGhlXh2tlc6iTc41PACqy\nyy+mHmSv\n=kkH7\n-----END PGP PUBLIC KEY BLOCK-----\n"
	RhelGPG   = "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBErgSTsBEACh2A4b0O9t+vzC9VrVtL1AKvUWi9OPCjkvR7Xd8DtJxeeMZ5eF\n0HtzIG58qDRybwUe89FZprB1ffuUKzdE+HcL3FbNWSSOXVjZIersdXyH3NvnLLLF\n0DNRB2ix3bXG9Rh/RXpFsNxDp2CEMdUvbYCzE79K1EnUTVh1L0Of023FtPSZXX0c\nu7Pb5DI5lX5YeoXO6RoodrIGYJsVBQWnrWw4xNTconUfNPk0EGZtEnzvH2zyPoJh\nXGF+Ncu9XwbalnYde10OCvSWAZ5zTCpoLMTvQjWpbCdWXJzCm6G+/hx9upke546H\n5IjtYm4dTIVTnc3wvDiODgBKRzOl9rEOCIgOuGtDxRxcQkjrC+xvg5Vkqn7vBUyW\n9pHedOU+PoF3DGOM+dqv+eNKBvh9YF9ugFAQBkcG7viZgvGEMGGUpzNgN7XnS1gj\n/DPo9mZESOYnKceve2tIC87p2hqjrxOHuI7fkZYeNIcAoa83rBltFXaBDYhWAKS1\nPcXS1/7JzP0ky7d0L6Xbu/If5kqWQpKwUInXtySRkuraVfuK3Bpa+X1XecWi24JY\nHVtlNX025xx1ewVzGNCTlWn1skQN2OOoQTV4C8/qFpTW6DTWYurd4+fE0OJFJZQF\nbuhfXYwmRlVOgN5i77NTIJZJQfYFj38c/Iv5vZBPokO6mffrOTv3MHWVgQARAQAB\ntDNSZWQgSGF0LCBJbmMuIChyZWxlYXNlIGtleSAyKSA8c2VjdXJpdHlAcmVkaGF0\nLmNvbT6JAjYEEwECACAFAkrgSTsCGwMGCwkIBwMCBBUCCAMEFgIDAQIeAQIXgAAK\nCRAZni+R/UMdUWzpD/9s5SFR/ZF3yjY5VLUFLMXIKUztNN3oc45fyLdTI3+UClKC\n2tEruzYjqNHhqAEXa2sN1fMrsuKec61Ll2NfvJjkLKDvgVIh7kM7aslNYVOP6BTf\nC/JJ7/ufz3UZmyViH/WDl+AYdgk3JqCIO5w5ryrC9IyBzYv2m0HqYbWfphY3uHw5\nun3ndLJcu8+BGP5F+ONQEGl+DRH58Il9Jp3HwbRa7dvkPgEhfFR+1hI+Btta2C7E\n0/2NKzCxZw7Lx3PBRcU92YKyaEihfy/aQKZCAuyfKiMvsmzs+4poIX7I9NQCJpyE\nIGfINoZ7VxqHwRn/d5mw2MZTJjbzSf+Um9YJyA0iEEyD6qjriWQRbuxpQXmlAJbh\n8okZ4gbVFv1F8MzK+4R8VvWJ0XxgtikSo72fHjwha7MAjqFnOq6eo6fEC/75g3NL\nGht5VdpGuHk0vbdENHMC8wS99e5qXGNDued3hlTavDMlEAHl34q2H9nakTGRF5Ki\nJUfNh3DVRGhg8cMIti21njiRh7gyFI2OccATY7bBSr79JhuNwelHuxLrCFpY7V25\nOFktl15jZJaMxuQBqYdBgSay2G0U6D1+7VsWufpzd/Abx1/c3oi9ZaJvW22kAggq\ndzdA27UUYjWvx42w9menJwh/0jeQcTecIUd0d0rFcw/c1pvgMMl/Q73yzKgKYw==\n=zbHE\n-----END PGP PUBLIC KEY BLOCK-----\n-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBFsy23UBEACUKSphFEIEvNpy68VeW4Dt6qv+mU6am9a2AAl10JANLj1oqWX+\noYk3en1S6cVe2qehSL5DGVa3HMUZkP3dtbD4SgzXzxPodebPcr4+0QNWigkUisri\nXGL5SCEcOP30zDhZvg+4mpO2jMi7Kc1DLPzBBkgppcX91wa0L1pQzBcvYMPyV/Dh\nKbQHR75WdkP6OA2JXdfC94nxYq+2e0iPqC1hCP3Elh+YnSkOkrawDPmoB1g4+ft/\nxsiVGVy/W0ekXmgvYEHt6si6Y8NwXgnTMqxeSXQ9YUgVIbTpsxHQKGy76T5lMlWX\n4LCOmEVomBJg1SqF6yi9Vu8TeNThaDqT4/DddYInd0OO69s0kGIXalVgGYiW2HOD\nx2q5R1VGCoJxXomz+EbOXY+HpKPOHAjU0DB9MxbU3S248LQ69nIB5uxysy0PSco1\nsdZ8sxRNQ9Dw6on0Nowx5m6Thefzs5iK3dnPGBqHTT43DHbnWc2scjQFG+eZhe98\nEll/kb6vpBoY4bG9/wCG9qu7jj9Z+BceCNKeHllbezVLCU/Hswivr7h2dnaEFvPD\nO4GqiWiwOF06XaBMVgxA8p2HRw0KtXqOpZk+o+sUvdPjsBw42BB96A1yFX4jgFNA\nPyZYnEUdP6OOv9HSjnl7k/iEkvHq/jGYMMojixlvXpGXhnt5jNyc4GSUJQARAQAB\ntDNSZWQgSGF0LCBJbmMuIChhdXhpbGlhcnkga2V5KSA8c2VjdXJpdHlAcmVkaGF0\nLmNvbT6JAjkEEwECACMFAlsy23UCGwMHCwkIBwMCAQYVCAIJCgsEFgIDAQIeAQIX\ngAAKCRD3b2bD1AgnknqOD/9fB2ASuG2aJIiap4kK58R+RmOVM4qgclAnaG57+vjI\nnKvyfV3NH/keplGNRxwqHekfPCqvkpABwhdGEXIE8ILqnPewIMr6PZNZWNJynZ9i\neSMzVuCG7jDoGyQ5/6B0f6xeBtTeBDiRl7+Alehet1twuGL1BJUYG0QuLgcEzkaE\n/gkuumeVcazLzz7L12D22nMk66GxmgXfqS5zcbqOAuZwaA6VgSEgFdV2X2JU79zS\nBQJXv7NKc+nDXFG7M7EHjY3Rma3HXkDbkT8bzh9tJV7Z7TlpT829pStWQyoxKCVq\nsEX8WsSapTKA3P9YkYCwLShgZu4HKRFvHMaIasSIZWzLu+RZH/4yyHOhj0QB7XMY\neHQ6fGSbtJ+K6SrpHOOsKQNAJ0hVbSrnA1cr5+2SDfel1RfYt0W9FA6DoH/S5gAR\ndzT1u44QVwwp3U+eFpHphFy//uzxNMtCjjdkpzhYYhOCLNkDrlRPb+bcoL/6ePSr\n016PA7eEnuC305YU1Ml2WcCn7wQV8x90o33klJmEkWtXh3X39vYtI4nCPIvZn1eP\nVy+F+wWt4vN2b8oOdlzc2paOembbCo2B+Wapv5Y9peBvlbsDSgqtJABfK8KQq/jK\nYl3h5elIa1I3uNfczeHOnf1enLOUOlq630yeM/yHizz99G1g+z/guMh5+x/OHraW\niA==\n=+Gxh\n-----END PGP PUBLIC KEY BLOCK-----\n"
	GcpGPG    = "-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n\nmQENBFWKtqgBCADmKQWYQF9YoPxLEQZ5XA6DFVg9ZHG4HIuehsSJETMPQ+W9K5c5\nUs5assCZBjG/k5i62SmWb09eHtWsbbEgexURBWJ7IxA8kM3kpTo7bx+LqySDsSC3\n/8JRkiyibVV0dDNv/EzRQsGDxmk5Xl8SbQJ/C2ECSUT2ok225f079m2VJsUGHG+5\nRpyHHgoMaRNedYP8ksYBPSD6sA3Xqpsh/0cF4sm8QtmsxkBmCCIjBa0B0LybDtdX\nXIq5kPJsIrC2zvERIPm1ez/9FyGmZKEFnBGeFC45z5U//pHdB1z03dYKGrKdDpID\n17kNbC5wl24k/IeYyTY9IutMXvuNbVSXaVtRABEBAAG0Okdvb2dsZSBDbG91ZCBQ\nYWNrYWdlcyBSUE0gU2lnbmluZyBLZXkgPGdjLXRlYW1AZ29vZ2xlLmNvbT6JATgE\nEwECACIFAlWKtqgCGy8GCwkIBwMCBhUIAgkKCwQWAgMBAh4BAheAAAoJEPCcOUw+\nG6jV+QwH/0wRH+XovIwLGfkg6kYLEvNPvOIYNQWnrT6zZ+XcV47WkJ+i5SR+QpUI\nudMSWVf4nkv+XVHruxydafRIeocaXY0E8EuIHGBSB2KR3HxG6JbgUiWlCVRNt4Qd\n6udC6Ep7maKEIpO40M8UHRuKrp4iLGIhPm3ELGO6uc8rks8qOBMH4ozU+3PB9a0b\nGnPBEsZdOBI1phyftLyyuEvG8PeUYD+uzSx8jp9xbMg66gQRMP9XGzcCkD+b8w1o\n7v3J3juKKpgvx5Lqwvwv2ywqn/Wr5d5OBCHEw8KtU/tfxycz/oo6XUIshgEbS/+P\n6yKDuYhRp6qxrYXjmAszIT25cftb4d4=\n=/PbX\n-----END PGP PUBLIC KEY BLOCK-----"
)

var (
	RepoBaseID   = "2531793b-c607-4e1c-80b2-fbbaf9d12790"
	RepoAppstrID = "dbd21dfc-1733-4877-b1c8-8fb5a98beeb4"
	RepoPLID     = "a7ec8864-0e3c-4af2-8c06-567891280af5"
	RepoPLID2    = "c01c2d9c-4624-4558-9ca9-8abcc5eb4437"
)

func rhRepos(urls []string) (res []content_sources.ApiRepositoryResponse) {
	if slices.Contains(urls, "https://cdn.redhat.com/content/dist/rhel9/9/x86_64/baseos/os") {
		res = append(res, content_sources.ApiRepositoryResponse{
			GpgKey:   common.ToPtr(RhelGPG),
			Uuid:     common.ToPtr(RepoBaseID),
			Snapshot: common.ToPtr(true),
			Name:     common.ToPtr("baseos"),
		})
	}

	if slices.Contains(urls, "https://cdn.redhat.com/content/dist/rhel9/9/x86_64/appstream/os") {
		res = append(res, content_sources.ApiRepositoryResponse{
			GpgKey:   common.ToPtr(RhelGPG),
			Uuid:     common.ToPtr(RepoAppstrID),
			Snapshot: common.ToPtr(true),
			Name:     common.ToPtr("appstream"),
		})
	}

	return res
}

func extRepos(urls []string) (res []content_sources.ApiRepositoryResponse) {
	if slices.Contains(urls, "https://some-repo-base-url.org") {
		res = append(res, content_sources.ApiRepositoryResponse{
			GpgKey:   common.ToPtr("some-gpg-key"),
			Uuid:     common.ToPtr(RepoPLID),
			Snapshot: common.ToPtr(true),
			Name:     common.ToPtr("payload"),
		})
	}

	if slices.Contains(urls, "https://some-repo-base-url2.org") {
		res = append(res, content_sources.ApiRepositoryResponse{
			Uuid:     common.ToPtr(RepoPLID2),
			Snapshot: common.ToPtr(true),
			Name:     common.ToPtr("payload2"),
		})
	}

	return res
}

func snaps(uuids []string) (res []content_sources.ApiSnapshotForDate) {
	if slices.Contains(uuids, RepoBaseID) {
		res = append(res, content_sources.ApiSnapshotForDate{
			IsAfter: common.ToPtr(false),
			Match: &content_sources.ApiSnapshotResponse{
				CreatedAt:      common.ToPtr("1998-01-30T00:00:00Z"),
				RepositoryPath: common.ToPtr("/snappy/baseos"),
				Url:            common.ToPtr("http://snappy-url/snappy/baseos"),
			},
			RepositoryUuid: common.ToPtr(RepoBaseID),
		})
	}

	if slices.Contains(uuids, RepoAppstrID) {
		res = append(res, content_sources.ApiSnapshotForDate{
			IsAfter: common.ToPtr(false),
			Match: &content_sources.ApiSnapshotResponse{
				CreatedAt:      common.ToPtr("1998-01-30T00:00:00Z"),
				RepositoryPath: common.ToPtr("/snappy/appstream"),
				Url:            common.ToPtr("http://snappy-url/snappy/appstream"),
			},
			RepositoryUuid: common.ToPtr(RepoAppstrID),
		})
	}

	if slices.Contains(uuids, RepoPLID) {
		res = append(res, content_sources.ApiSnapshotForDate{
			IsAfter: common.ToPtr(false),
			Match: &content_sources.ApiSnapshotResponse{
				CreatedAt:      common.ToPtr("1998-01-30T00:00:00Z"),
				RepositoryPath: common.ToPtr("/snappy/payload"),
				Url:            common.ToPtr("http://snappy-url/snappy/payload"),
			},
			RepositoryUuid: common.ToPtr(RepoPLID),
		})
	}

	if slices.Contains(uuids, RepoPLID2) {
		res = append(res, content_sources.ApiSnapshotForDate{
			IsAfter: common.ToPtr(false),
			Match: &content_sources.ApiSnapshotResponse{
				CreatedAt:      common.ToPtr("1998-01-30T00:00:00Z"),
				RepositoryPath: common.ToPtr("/snappy/payload2"),
				Url:            common.ToPtr("http://snappy-url/snappy/payload2"),
			},
			RepositoryUuid: common.ToPtr(RepoPLID2),
		})
	}
	return res
}

func exports(uuids []string) (res []content_sources.ApiRepositoryExportResponse) {
	if slices.Contains(uuids, RepoBaseID) {
		res = append(res, content_sources.ApiRepositoryExportResponse{
			GpgKey: common.ToPtr(RhelGPG),
			Name:   common.ToPtr("baseos"),
			Url:    common.ToPtr("http://snappy-url/snappy/baseos"),
		})
	}
	if slices.Contains(uuids, RepoAppstrID) {
		res = append(res, content_sources.ApiRepositoryExportResponse{
			GpgKey: common.ToPtr(RhelGPG),
			Name:   common.ToPtr("appstream"),
			Url:    common.ToPtr("http://snappy-url/snappy/appstream"),
		})
	}
	if slices.Contains(uuids, RepoPLID) {
		res = append(res, content_sources.ApiRepositoryExportResponse{
			GpgKey: common.ToPtr("some-gpg-key"),
			Name:   common.ToPtr("payload"),
			Url:    common.ToPtr("http://snappy-url/snappy/payload"),
		})
	}
	if slices.Contains(uuids, RepoPLID2) {
		res = append(res, content_sources.ApiRepositoryExportResponse{
			GpgKey: common.ToPtr("some-gpg-key"),
			Name:   common.ToPtr("payload2"),
			Url:    common.ToPtr("http://snappy-url/snappy/payload2"),
		})
	}
	return res
}

func ContentSources(w http.ResponseWriter, r *http.Request) {
	if tutils.AuthString0 != r.Header.Get("x-rh-identity") {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/repositories/":
		urlForm := r.URL.Query().Get("url")
		urls := strings.Split(urlForm, ",")

		var repos []content_sources.ApiRepositoryResponse
		switch r.URL.Query().Get("origin") {
		case "red_hat":
			repos = append(repos, rhRepos(urls)...)
		case "external":
			repos = append(repos, extRepos(urls)...)
		}
		err := json.NewEncoder(w).Encode(content_sources.ApiRepositoryCollectionResponse{
			Data: &repos,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "/snapshots/for_date/":
		var body content_sources.ApiListSnapshotByDateRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if "1999-01-30T00:00:00Z" != body.Date {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(content_sources.ApiListSnapshotByDateResponse{
			Data: common.ToPtr(snaps(body.RepositoryUuids)),
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "/repositories/bulk_export/":
		var body content_sources.ApiRepositoryExportRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(exports(body.RepositoryUuids))
		if err != nil {
			w.WriteHeader(http.StatusInsufficientStorage)
			return
		}
	}
}
