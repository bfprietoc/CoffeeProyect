-- Seed: productores
INSERT INTO producers (id, name) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Diego Samuel Bermúdez'),
    ('a0000000-0000-0000-0000-000000000002', 'Camilo Merizalde'),
    ('a0000000-0000-0000-0000-000000000003', 'Familia Muñoz'),
    ('a0000000-0000-0000-0000-000000000004', 'Asociación Cafeteros de Nariño'),
    ('a0000000-0000-0000-0000-000000000005', 'Cooperativa Andes')
ON CONFLICT DO NOTHING;

-- Seed: fincas
INSERT INTO farms (id, name, country, region, producer_id) VALUES
    ('b0000000-0000-0000-0000-000000000001', 'El Paraíso',        'Colombia', 'Cauca',            'a0000000-0000-0000-0000-000000000001'),
    ('b0000000-0000-0000-0000-000000000002', 'La Esperanza',      'Colombia', 'Valle del Cauca',  'a0000000-0000-0000-0000-000000000002'),
    ('b0000000-0000-0000-0000-000000000003', 'Finca El Oasis',    'Colombia', 'Huila',            'a0000000-0000-0000-0000-000000000003'),
    ('b0000000-0000-0000-0000-000000000004', 'Alto de la Cruz',   'Colombia', 'Nariño',           'a0000000-0000-0000-0000-000000000004'),
    ('b0000000-0000-0000-0000-000000000005', 'Varias fincas',     'Colombia', 'Antioquia',        'a0000000-0000-0000-0000-000000000005')
ON CONFLICT DO NOTHING;

-- Seed: cafés
INSERT INTO coffees (id, name, process, roast_level, tasting_notes, description, farm_id, bag_size_grams, price_cents, currency, stock_bags) VALUES
    (
        'c1a2b3c4-0001-0001-0001-000000000001',
        'El Paraíso 92',
        'anaerobic', 'light',
        '["maracuyá", "uva", "chocolate negro"]',
        'Café de proceso anaeróbico de la finca El Paraíso en el Cauca. Fermentación controlada de 48h que potencia sus notas afrutadas y una acidez vibrante.',
        'b0000000-0000-0000-0000-000000000001',
        250, 8500000, 'COP', 12
    ),
    (
        'c1a2b3c4-0002-0002-0002-000000000002',
        'Finca La Esperanza Wush Wush',
        'washed', 'light',
        '["jazmín", "durazno", "té negro"]',
        'Variedad Wush Wush de origen etíope cultivada en el Valle del Cauca. Proceso lavado que resalta su delicada acidez floral y cuerpo sedoso.',
        'b0000000-0000-0000-0000-000000000002',
        250, 9200000, 'COP', 8
    ),
    (
        'c1a2b3c4-0003-0003-0003-000000000003',
        'Huila Natural Caturra',
        'natural', 'medium',
        '["panela", "cereza", "almendra"]',
        'Caturra de proceso natural del Huila. Secado en camas africanas durante 25 días. Dulzura prominente con cuerpo redondo y un final largo a frutos rojos.',
        'b0000000-0000-0000-0000-000000000003',
        340, 5500000, 'COP', 25
    ),
    (
        'c1a2b3c4-0004-0004-0004-000000000004',
        'Nariño Honey Gesha',
        'honey', 'light',
        '["bergamota", "mango", "miel de caña"]',
        'Gesha en proceso honey de los altiplanos de Nariño a 2.100 msnm. Mucílago retenido al 50% para una dulzura equilibrada con la acidez brillante característica del terroir nariñense.',
        'b0000000-0000-0000-0000-000000000004',
        250, 11000000, 'COP', 6
    ),
    (
        'c1a2b3c4-0005-0005-0005-000000000005',
        'Antioquia Dark Roast Blend',
        'washed', 'dark',
        '["chocolate amargo", "caramelo", "nuez"]',
        'Blend de fincas del suroeste antioqueño con tueste oscuro. Diseñado para espresso y métodos de alta presión. Cuerpo denso y dulzura de caramelo sin acidez invasiva.',
        'b0000000-0000-0000-0000-000000000005',
        500, 4800000, 'COP', 0
    )
ON CONFLICT DO NOTHING;
