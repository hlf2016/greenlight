package mocks

import "greenlight.311102.xyz/internal/data"

type MockMovieModel struct{}

func (m MockMovieModel) Insert(movie *data.Movie) error {
	return nil
}

func (m MockMovieModel) Get(id int64) (*data.Movie, error) {
	return nil, nil
}

func (m MockMovieModel) Update(movie *data.Movie) error {
	return nil
}

func (m MockMovieModel) Delete(id int64) error {
	return nil
}
