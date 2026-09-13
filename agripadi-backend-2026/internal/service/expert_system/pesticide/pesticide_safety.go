package pesticide

import "github.com/gustian305/backend/internal/domain"

// SafetyService hanya menampilkan arahan keselamatan yang dapat diterapkan
// secara umum. Informasi khusus seperti kelas bahaya, masa tunggu, waktu masuk
// kembali, dan batas jumlah aplikasi harus berasal dari label resmi setiap
// produk. Dataset saat ini belum menyimpan rincian label tersebut, sehingga
// sistem tidak boleh mengarang angka atau kelas bahaya.
type SafetyService struct{}

func NewSafetyService() *SafetyService {
	return &SafetyService{}
}

type SafetyResult struct {
	ToxicityClass       string
	EnvironmentalImpact string
	PollinatorRisk      string
	WorkerRisk          string
	Warnings            []string
	Recommendations     []string
	PreHarvestInterval  string
	ReentryInterval     string
	MaxApplication      string
	IsSafe              bool
	IsRestricted        bool
}

func (s *SafetyService) Analyze(recommendation Recommendation) SafetyResult {
	return s.AnalyzeProduct(recommendation.Pesticide)
}

func (s *SafetyService) AnalyzeProduct(product domain.Pesticide) SafetyResult {
	_ = s
	_ = product

	return SafetyResult{
		ToxicityClass:       "Periksa label produk",
		EnvironmentalImpact: "Periksa label produk",
		PollinatorRisk:      "Periksa label produk",
		WorkerRisk:          "Periksa label produk",
		Warnings: []string{
			"Informasi bahaya khusus produk belum tersimpan lengkap. Periksa label resmi sebelum digunakan.",
			"Jangan mencampur produk atau bahan aktif tanpa petunjuk pada label atau arahan petugas pertanian.",
		},
		Recommendations: []string{
			"Gunakan sarung tangan, masker, pakaian lengan panjang, celana panjang, dan alas kaki tertutup.",
			"Hindari penyemprotan saat hujan, angin kencang, atau ketika ada orang dan hewan di sekitar lahan.",
			"Jangan makan, minum, atau merokok saat menyiapkan dan menggunakan pestisida.",
			"Cuci alat dan diri setelah aplikasi, lalu simpan produk jauh dari anak-anak, makanan, pakan, dan sumber air.",
		},
		PreHarvestInterval: "Ikuti masa tunggu sebelum panen pada label resmi produk",
		ReentryInterval:    "Ikuti waktu aman untuk masuk kembali ke lahan pada label resmi produk",
		MaxApplication:     "Ikuti batas jumlah aplikasi pada label resmi produk",
		IsSafe:             false,
		IsRestricted:       false,
	}
}

// Dataset AgriPadi belum memuat data label keselamatan per produk yang cukup
// untuk membandingkan risiko secara sahih. Nilai netral mencegah produk mendapat
// peringkat lebih tinggi hanya karena asumsi bahan aktif yang tidak lengkap.
func (s *SafetyService) ScoreProduct(product domain.Pesticide) float64 {
	_ = s

	if len(product.Ingredients) == 0 {
		return 0.40
	}

	return 0.50
}
