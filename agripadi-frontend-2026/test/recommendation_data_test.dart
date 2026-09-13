import 'package:agripadi/features/chats/models/recommendation_data.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('RecommendationData dosage formatting', () {
    test('keeps the dose unit on selected value and label range', () {
      final data = RecommendationData.fromDiagnosis({
        'recommendations': [
          {
            'display_name': 'Produk Uji',
            'dosage': {
              'dose_raw': '1.125; nilai tengah rentang label 0.75-1.5',
              'dose_unit': 'ml/l',
            },
          },
        ],
      });

      expect(data.pesticides, hasLength(1));
      expect(
        data.pesticides.single,
        contains(
          'Dosis: 1.125 ml/l '
          '(nilai tengah dari rentang label 0.75-1.5 ml/l)',
        ),
      );
    });

    test('adds a simple unit when the raw dose has no unit', () {
      final data = RecommendationData.fromDiagnosis({
        'recommendations': [
          {
            'display_name': 'Produk Uji',
            'dosage': {
              'dose_raw': '200',
              'dose_unit': 'g/ha',
            },
          },
        ],
      });

      expect(data.pesticides.single, contains('Dosis: 200 g/ha'));
    });
  });
}
