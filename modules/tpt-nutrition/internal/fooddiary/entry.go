// Package fooddiary provides food diary entry types for dietetics and nutrition.
package fooddiary

type FoodDiaryEntry struct {
	ID          string `json:"id"`
	PatientNHI  string `json:"patientNhi"`
	Date        int64  `json:"date"`        // Unix epoch ms for meal date
	MealType    string `json:"mealType"`    // breakfast, lunch, dinner, snack1, snack2
	FoodItem    string `json:"foodItem"`
	Portion     string `json:"portion"`
	Calories    int    `json:"calories"`
	ProteinG    float64 `json:"proteinG"`
	CarbsG      float64 `json:"carbsG"`
	FatG        float64 `json:"fatG"`
	FiberG      float64 `json:"fiberG"`
	SugarG      float64 `json:"sugarG"`
	SodiumMg    float64 `json:"sodiumMg"`
	Notes       string `json:"notes,omitempty"`
	ClinicianID string `json:"clinicianId"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}