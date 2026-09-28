package domain

import "errors"

var IncorrectTripDepartureTime = errors.New("недопустимое время начало поездки")
var IncorrectTripSeats = errors.New("недопустимое кол-во слотов в поездке")
