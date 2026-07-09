// Package mealplan provides meal planning for dietetics and nutrition.
package mealplan

type MealPlan struct {
	ID            string     `json:"id"`
	PatientNHI    string     `json:"patientNhi"`
	ClinicianID   string     `json:"clinicianId"`
	PracticeID    string     `json:"practiceId"`
	Name          string     `json:"name"`
	Goal          string     `json:"goal"` // weight_loss, weight_gain, maintenance, therapeutic
	CalorieTarget int        `json:"calorieTarget"`
	DurationDays  int        `json:"durationDays"`
	Meals         []MealSlot `json:"meals"`
	Restrictions  string     `json:"restrictions,omitempty"`
	Notes         string     `json:"notes,omitempty"`
	Active        bool       `json:"active"`
	CreatedAt     int64      `json:"createdAt"`
	UpdatedAt     int64      `json:"updatedAt"`
}

type MealSlot struct {
	Day       int    `json:"day"`
	MealType  string `json:"mealType"` // breakfast, lunch, dinner, snack
	FoodItems string `json:"foodItems"`
	Portions  string `json:"portions"`
	Calories  int    `json:"calories"`
}
